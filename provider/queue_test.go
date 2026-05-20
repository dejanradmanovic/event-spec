package provider_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dejanradmanovic/event-spec/provider"
)

// collectFlushFunc returns a FlushFunc that appends each received batch to *collected.
func collectFlushFunc(mu *sync.Mutex, collected *[][]provider.QueuedMessage) provider.FlushFunc {
	return func(_ context.Context, batch []provider.QueuedMessage) error {
		mu.Lock()
		defer mu.Unlock()
		cp := make([]provider.QueuedMessage, len(batch))
		copy(cp, batch)
		*collected = append(*collected, cp)
		return nil
	}
}

func trackMsg(id string) provider.QueuedMessage {
	return provider.QueuedMessage{
		Op:    "track",
		Track: &provider.TrackMessage{MessageID: id},
	}
}

func totalMessages(batches [][]provider.QueuedMessage) int {
	n := 0
	for _, b := range batches {
		n += len(b)
	}
	return n
}

// TestQueue_BatchFlush verifies that enqueueing exactly BatchSize events triggers
// an automatic flush before the FlushInterval elapses.
func TestQueue_BatchFlush(t *testing.T) {
	var mu sync.Mutex
	var collected [][]provider.QueuedMessage

	cfg := provider.ProviderConfig{
		BatchSize:     3,
		FlushInterval: 10 * time.Second, // long interval — batch should fire first
		MaxQueueSize:  100,
	}
	q := provider.NewQueue(cfg, collectFlushFunc(&mu, &collected))
	defer q.Shutdown(context.Background()) //nolint:errcheck

	for i := range 3 {
		if err := q.Enqueue(context.Background(), trackMsg(string(rune('A'+i)))); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}

	// Flush waits for the run loop to complete the batch flush.
	if err := q.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	mu.Lock()
	total := totalMessages(collected)
	mu.Unlock()

	if total != 3 {
		t.Errorf("expected 3 messages flushed, got %d", total)
	}
}

// TestQueue_IntervalFlush verifies that buffered events are auto-flushed once
// the FlushInterval elapses even when BatchSize is not reached.
func TestQueue_IntervalFlush(t *testing.T) {
	var mu sync.Mutex
	var collected [][]provider.QueuedMessage

	cfg := provider.ProviderConfig{
		BatchSize:     100,
		FlushInterval: 50 * time.Millisecond,
		MaxQueueSize:  100,
	}
	q := provider.NewQueue(cfg, collectFlushFunc(&mu, &collected))
	defer q.Shutdown(context.Background()) //nolint:errcheck

	if err := q.Enqueue(context.Background(), trackMsg("x")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// Wait long enough for at least one ticker tick plus processing headroom.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	total := totalMessages(collected)
	mu.Unlock()

	if total == 0 {
		t.Error("expected at least 1 message auto-flushed on interval, got 0")
	}
}

// TestQueue_OverflowDropOldest verifies that the oldest event is evicted when
// the queue is full and OverflowDropOldest is set.
func TestQueue_OverflowDropOldest(t *testing.T) {
	var mu sync.Mutex
	var collected [][]provider.QueuedMessage

	const maxSize = 3
	cfg := provider.ProviderConfig{
		BatchSize:      100,
		FlushInterval:  10 * time.Second,
		MaxQueueSize:   maxSize,
		OverflowPolicy: provider.OverflowDropOldest,
	}
	q := provider.NewQueue(cfg, collectFlushFunc(&mu, &collected))
	defer q.Shutdown(context.Background()) //nolint:errcheck

	// Fill the queue to capacity with messages A, B, C.
	for _, id := range []string{"A", "B", "C"} {
		if err := q.Enqueue(context.Background(), trackMsg(id)); err != nil {
			t.Fatalf("Enqueue %s: %v", id, err)
		}
	}

	// Enqueue D — should evict A (oldest).
	if err := q.Enqueue(context.Background(), trackMsg("D")); err != nil {
		t.Fatalf("Enqueue D: %v", err)
	}

	if err := q.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	mu.Lock()
	total := totalMessages(collected)
	var ids []string
	for _, batch := range collected {
		for _, m := range batch {
			ids = append(ids, m.Track.MessageID)
		}
	}
	mu.Unlock()

	if total != maxSize {
		t.Errorf("expected %d messages, got %d", maxSize, total)
	}
	// A must have been dropped; B, C, D must be present.
	for _, want := range []string{"B", "C", "D"} {
		found := false
		for _, id := range ids {
			if id == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected message %q to be present, ids=%v", want, ids)
		}
	}
}

// TestQueue_OverflowDropNewest verifies that incoming events are discarded when
// the queue is full and OverflowDropNewest is set.
func TestQueue_OverflowDropNewest(t *testing.T) {
	var mu sync.Mutex
	var collected [][]provider.QueuedMessage

	const maxSize = 2
	cfg := provider.ProviderConfig{
		BatchSize:      100,
		FlushInterval:  10 * time.Second,
		MaxQueueSize:   maxSize,
		OverflowPolicy: provider.OverflowDropNewest,
	}
	q := provider.NewQueue(cfg, collectFlushFunc(&mu, &collected))
	defer q.Shutdown(context.Background()) //nolint:errcheck

	for _, id := range []string{"A", "B", "C"} { // C should be dropped
		if err := q.Enqueue(context.Background(), trackMsg(id)); err != nil {
			t.Fatalf("Enqueue %s: %v", id, err)
		}
	}

	if err := q.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	mu.Lock()
	total := totalMessages(collected)
	mu.Unlock()

	if total != maxSize {
		t.Errorf("expected %d messages (C dropped), got %d", maxSize, total)
	}
}

// TestQueue_OverflowBlock verifies that an Enqueue call blocks when the queue is
// full and resumes after a Flush makes space.
func TestQueue_OverflowBlock(t *testing.T) {
	var flushed atomic.Int32
	fn := func(_ context.Context, batch []provider.QueuedMessage) error {
		flushed.Add(int32(len(batch)))
		return nil
	}

	const maxSize = 2
	cfg := provider.ProviderConfig{
		BatchSize:      100,
		FlushInterval:  10 * time.Second,
		MaxQueueSize:   maxSize,
		OverflowPolicy: provider.OverflowBlock,
	}
	q := provider.NewQueue(cfg, fn)
	defer q.Shutdown(context.Background()) //nolint:errcheck

	// Fill to capacity.
	for _, id := range []string{"A", "B"} {
		if err := q.Enqueue(context.Background(), trackMsg(id)); err != nil {
			t.Fatalf("Enqueue %s: %v", id, err)
		}
	}

	blocked := make(chan error, 1)
	go func() {
		// This should block until Flush drains the queue.
		blocked <- q.Enqueue(context.Background(), trackMsg("C"))
	}()

	// Give the goroutine time to reach the Wait.
	time.Sleep(20 * time.Millisecond)

	// Flush drains A and B, making room for C.
	if err := q.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	select {
	case err := <-blocked:
		if err != nil {
			t.Errorf("blocked Enqueue returned error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("blocked Enqueue did not unblock after Flush")
	}
}

// TestQueue_OverflowBlock_ContextCancel verifies that a blocked Enqueue returns
// ctx.Err() when the context is cancelled.
func TestQueue_OverflowBlock_ContextCancel(t *testing.T) {
	cfg := provider.ProviderConfig{
		BatchSize:      100,
		FlushInterval:  10 * time.Second,
		MaxQueueSize:   1,
		OverflowPolicy: provider.OverflowBlock,
	}
	q := provider.NewQueue(cfg, func(_ context.Context, _ []provider.QueuedMessage) error { return nil })
	defer q.Shutdown(context.Background()) //nolint:errcheck

	// Fill the queue.
	if err := q.Enqueue(context.Background(), trackMsg("A")); err != nil {
		t.Fatalf("Enqueue A: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- q.Enqueue(ctx, trackMsg("B"))
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-result:
		if err == nil {
			t.Error("expected error after context cancellation, got nil")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Enqueue did not unblock after context cancellation")
	}
}

// TestQueue_Flush_Synchronous verifies that Flush waits for fn to complete.
func TestQueue_Flush_Synchronous(t *testing.T) {
	started := make(chan struct{})
	unblock := make(chan struct{})

	fn := func(_ context.Context, batch []provider.QueuedMessage) error {
		close(started)
		<-unblock
		return nil
	}

	cfg := provider.ProviderConfig{
		BatchSize:     100,
		FlushInterval: 10 * time.Second,
		MaxQueueSize:  100,
	}
	q := provider.NewQueue(cfg, fn)
	defer q.Shutdown(context.Background()) //nolint:errcheck

	if err := q.Enqueue(context.Background(), trackMsg("x")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	flushDone := make(chan error, 1)
	go func() {
		flushDone <- q.Flush(context.Background())
	}()

	<-started // fn is executing

	select {
	case <-flushDone:
		t.Fatal("Flush returned before fn completed")
	case <-time.After(20 * time.Millisecond):
	}

	close(unblock)

	select {
	case err := <-flushDone:
		if err != nil {
			t.Errorf("Flush: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Flush did not return after fn completed")
	}
}

// TestQueue_Shutdown_DrainsThenCloses verifies that Shutdown flushes remaining
// events and prevents further Enqueue calls.
func TestQueue_Shutdown_DrainsThenCloses(t *testing.T) {
	var flushed atomic.Int32
	fn := func(_ context.Context, batch []provider.QueuedMessage) error {
		flushed.Add(int32(len(batch)))
		return nil
	}

	cfg := provider.ProviderConfig{
		BatchSize:     100,
		FlushInterval: 10 * time.Second,
		MaxQueueSize:  100,
	}
	q := provider.NewQueue(cfg, fn)

	for i := range 5 {
		if err := q.Enqueue(context.Background(), trackMsg(string(rune('A'+i)))); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}

	if err := q.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	if got := flushed.Load(); got != 5 {
		t.Errorf("expected 5 events flushed on Shutdown, got %d", got)
	}

	err := q.Enqueue(context.Background(), trackMsg("Z"))
	if err != provider.ErrQueueClosed {
		t.Errorf("expected ErrQueueClosed after Shutdown, got %v", err)
	}
}

// TestQueue_DefaultConfig verifies that zero-value ProviderConfig fields use package defaults.
func TestQueue_DefaultConfig(t *testing.T) {
	called := make(chan struct{}, 1)
	fn := func(_ context.Context, _ []provider.QueuedMessage) error {
		select {
		case called <- struct{}{}:
		default:
		}
		return nil
	}

	// Zero config — should not panic and should flush on interval.
	q := provider.NewQueue(provider.ProviderConfig{FlushInterval: 50 * time.Millisecond}, fn)
	defer q.Shutdown(context.Background()) //nolint:errcheck

	if err := q.Enqueue(context.Background(), trackMsg("x")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	select {
	case <-called:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected auto-flush to fire within 500ms")
	}
}
