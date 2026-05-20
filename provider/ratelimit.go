package provider

import (
	"context"
	"sync"
	"time"
)

// defaultRateLimitRPS is the fallback when RateLimitConfig.RequestsPerSecond is zero.
// 30 matches Amplitude's documented HTTP API limit.
const defaultRateLimitRPS = 30

// RateLimiter is a token-bucket rate limiter for per-provider API request throttling.
//
// Use Allow for non-blocking drop-on-exceed behaviour; use Wait to block the caller
// until a token is available. Both methods are safe for concurrent use.
type RateLimiter struct {
	rate     float64 // tokens per second
	burst    float64 // maximum token count (bucket capacity)
	tokens   float64
	lastTime time.Time
	mu       sync.Mutex
}

// NewRateLimiter constructs a RateLimiter from cfg.
// Zero-value fields fall back to defaults: RequestsPerSecond=30 (Amplitude limit),
// BurstSize=RequestsPerSecond*2.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	rps := cfg.RequestsPerSecond
	if rps <= 0 {
		rps = defaultRateLimitRPS
	}
	burst := cfg.BurstSize
	if burst <= 0 {
		burst = rps * 2
	}
	return &RateLimiter{
		rate:     float64(rps),
		burst:    float64(burst),
		tokens:   float64(burst), // start with a full bucket
		lastTime: time.Now(),
	}
}

// Allow reports whether a token is available and consumes it without blocking.
// Returns false when the bucket is empty; callers should drop the event.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refill(time.Now())
	if r.tokens < 1 {
		return false
	}
	r.tokens--
	return true
}

// Wait blocks until a token is available or ctx is cancelled.
// Returns ctx.Err() if the context is cancelled before a token becomes available.
func (r *RateLimiter) Wait(ctx context.Context) error {
	for {
		r.mu.Lock()
		r.refill(time.Now())
		if r.tokens >= 1 {
			r.tokens--
			r.mu.Unlock()
			return nil
		}
		// Duration until at least one token accrues from the current level.
		need := 1 - r.tokens
		delay := time.Duration(need / r.rate * float64(time.Second))
		r.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

// refill adds tokens earned since lastTime, capping at burst capacity.
// Must be called with r.mu held.
func (r *RateLimiter) refill(now time.Time) {
	elapsed := now.Sub(r.lastTime).Seconds()
	if elapsed > 0 {
		r.tokens += elapsed * r.rate
		if r.tokens > r.burst {
			r.tokens = r.burst
		}
		r.lastTime = now
	}
}
