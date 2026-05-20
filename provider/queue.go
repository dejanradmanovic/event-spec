package provider

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	defaultBatchSize     = 100
	defaultFlushInterval = 10 * time.Second
	defaultMaxQueueSize  = 10_000
)

// ErrQueueClosed is returned by Enqueue after Shutdown has been called.
var ErrQueueClosed = errors.New("provider: queue is closed")

// QueuedMessage is a tagged-union holding one analytics call ready for batch dispatch.
// Exactly one of Track/Identify/Group/Page/Alias is non-nil, corresponding to Op.
type QueuedMessage struct {
	Op       string // "track" | "identify" | "group" | "page" | "alias"
	Track    *TrackMessage
	Identify *IdentifyMessage
	Group    *GroupMessage
	Page     *PageMessage
	Alias    *AliasMessage
}

// FlushFunc is invoked by the queue to dispatch a batch of messages to the provider backend.
// Implementations must honour ctx deadlines.
type FlushFunc func(ctx context.Context, batch []QueuedMessage) error

// Queue is a per-provider in-memory event buffer with configurable batching,
// periodic auto-flush, and overflow handling.
//
// Start it with NewQueue; stop it with Shutdown. All methods are safe for concurrent use.
type Queue struct {
	batchSize int
	interval  time.Duration
	maxSize   int
	overflow  OverflowPolicy
	fn        FlushFunc

	mu     sync.Mutex
	cond   *sync.Cond
	buf    []QueuedMessage
	closed bool

	// flushCh carries flush requests to the run loop; capacity 1 means at most one
	// pending trigger is queued — the run loop drains the entire buffer each time.
	flushCh chan flushReq
	stopCh  chan struct{}
	wg      sync.WaitGroup

	shutdownOnce sync.Once
}

type flushReq struct {
	ctx  context.Context
	done chan<- error // nil = fire-and-forget
}

// NewQueue creates a Queue backed by fn and starts its background flush goroutine.
// Zero-value queue fields in cfg fall back to package defaults:
// BatchSize=100, FlushInterval=10s, MaxQueueSize=10000, OverflowPolicy=drop_oldest.
func NewQueue(cfg ProviderConfig, fn FlushFunc) *Queue {
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	interval := cfg.FlushInterval
	if interval <= 0 {
		interval = defaultFlushInterval
	}
	maxSize := cfg.MaxQueueSize
	if maxSize <= 0 {
		maxSize = defaultMaxQueueSize
	}
	overflow := cfg.OverflowPolicy
	if overflow == "" {
		overflow = OverflowDropOldest
	}

	q := &Queue{
		batchSize: batchSize,
		interval:  interval,
		maxSize:   maxSize,
		overflow:  overflow,
		fn:        fn,
		buf:       make([]QueuedMessage, 0, batchSize),
		flushCh:   make(chan flushReq, 1),
		stopCh:    make(chan struct{}),
	}
	q.cond = sync.NewCond(&q.mu)

	q.wg.Add(1)
	go q.run()

	return q
}

// Enqueue adds msg to the buffer. Overflow behaviour depends on OverflowPolicy:
//   - OverflowDropOldest: oldest buffered event is silently evicted to make room.
//   - OverflowDropNewest: msg is silently discarded; nil is returned.
//   - OverflowBlock: caller blocks until space is available or ctx is cancelled.
//
// When the buffer reaches BatchSize after the append, the run loop is notified
// for an immediate batch flush (non-blocking signal).
// Returns ErrQueueClosed if Shutdown has already been called.
func (q *Queue) Enqueue(ctx context.Context, msg QueuedMessage) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrQueueClosed
	}

	switch q.overflow {
	case OverflowDropOldest:
		if len(q.buf) >= q.maxSize {
			q.buf = q.buf[1:]
		}

	case OverflowDropNewest:
		if len(q.buf) >= q.maxSize {
			return nil
		}

	case OverflowBlock:
		// When ctx is cancellable, start a goroutine that broadcasts on cancellation
		// so the cond.Wait() below wakes up to check ctx.Err().
		if ctx.Done() != nil {
			watchStop := make(chan struct{})
			go func() {
				select {
				case <-ctx.Done():
					q.cond.Broadcast()
				case <-watchStop:
				}
			}()
			defer close(watchStop)
		}

		for len(q.buf) >= q.maxSize && !q.closed {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			q.cond.Wait() // releases mu, waits for Broadcast, reacquires mu
		}
		if q.closed {
			return ErrQueueClosed
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	q.buf = append(q.buf, msg)

	// Notify the run loop for an immediate flush when the batch threshold is met.
	// The channel is buffered(1); if a trigger is already pending, skip — one flush
	// will drain the entire buffer regardless.
	if len(q.buf) >= q.batchSize {
		select {
		case q.flushCh <- flushReq{ctx: context.Background()}:
		default:
		}
	}

	return nil
}

// Flush drains all currently buffered events synchronously by sending an explicit
// request to the run loop and waiting for completion.
// Returns ErrQueueClosed if Shutdown has been called.
func (q *Queue) Flush(ctx context.Context) error {
	done := make(chan error, 1)
	req := flushReq{ctx: ctx, done: done}

	select {
	case q.flushCh <- req:
	case <-ctx.Done():
		return ctx.Err()
	case <-q.stopCh:
		return ErrQueueClosed
	}

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Shutdown flushes remaining buffered events and stops the background goroutine.
// Subsequent Enqueue calls return ErrQueueClosed. Safe to call multiple times.
func (q *Queue) Shutdown(ctx context.Context) error {
	var flushErr error
	q.shutdownOnce.Do(func() {
		// Drain remaining events before stopping the run loop.
		flushErr = q.Flush(ctx)

		// Mark closed so Enqueue returns ErrQueueClosed from now on.
		q.mu.Lock()
		q.closed = true
		q.cond.Broadcast() // wake any Enqueue callers blocked on OverflowBlock
		q.mu.Unlock()

		close(q.stopCh)
		q.wg.Wait()
	})
	return flushErr
}

// Len returns the number of messages currently buffered. Intended for testing.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.buf)
}

// run is the background goroutine. It flushes on the ticker interval and on demand.
func (q *Queue) run() {
	defer q.wg.Done()

	ticker := time.NewTicker(q.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = q.doFlush(context.Background())

		case req := <-q.flushCh:
			ctx := req.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			err := q.doFlush(ctx)
			ticker.Reset(q.interval) // avoid redundant re-flush right after an explicit drain
			if req.done != nil {
				req.done <- err
			}

		case <-q.stopCh:
			return
		}
	}
}

// doFlush atomically takes the current buffer and dispatches it via fn.
func (q *Queue) doFlush(ctx context.Context) error {
	q.mu.Lock()
	if len(q.buf) == 0 {
		q.mu.Unlock()
		return nil
	}
	batch := q.buf
	q.buf = make([]QueuedMessage, 0, q.batchSize)
	if q.overflow == OverflowBlock {
		q.cond.Broadcast() // space freed; wake any blocked Enqueue callers
	}
	q.mu.Unlock()

	return q.fn(ctx, batch)
}
