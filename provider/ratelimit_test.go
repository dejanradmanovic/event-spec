package provider_test

import (
	"context"
	"testing"
	"time"

	"event-spec/provider"
)

// TestRateLimiter_DefaultConfig verifies that a zero-value RateLimitConfig uses package
// defaults and the limiter is immediately usable.
func TestRateLimiter_DefaultConfig(t *testing.T) {
	rl := provider.NewRateLimiter(provider.RateLimitConfig{})
	if !rl.Allow() {
		t.Error("Allow(): expected true on fresh limiter with default config")
	}
}

// TestRateLimiter_Allow_BurstThenThrottle verifies that the bucket honours burst capacity
// and denies requests once the bucket is empty.
func TestRateLimiter_Allow_BurstThenThrottle(t *testing.T) {
	const burst = 3
	rl := provider.NewRateLimiter(provider.RateLimitConfig{
		RequestsPerSecond: 1,
		BurstSize:         burst,
	})

	for i := range burst {
		if !rl.Allow() {
			t.Fatalf("Allow() call %d: expected true (within burst)", i+1)
		}
	}

	if rl.Allow() {
		t.Error("Allow(): expected false after burst exhausted")
	}
}

// TestRateLimiter_Refill verifies that tokens refill after the token interval elapses.
func TestRateLimiter_Refill(t *testing.T) {
	// 10 req/s → one token per 100 ms.
	rl := provider.NewRateLimiter(provider.RateLimitConfig{
		RequestsPerSecond: 10,
		BurstSize:         1,
	})

	if !rl.Allow() {
		t.Fatal("Allow(): expected true on fresh limiter")
	}
	if rl.Allow() {
		t.Fatal("Allow(): expected false after burst exhausted")
	}

	// Wait for more than one token interval.
	time.Sleep(150 * time.Millisecond)

	if !rl.Allow() {
		t.Error("Allow(): expected true after token refill interval")
	}
}

// TestRateLimiter_Wait_ThrottlesCorrectly verifies that Wait blocks for approximately
// the token refill duration before returning.
func TestRateLimiter_Wait_ThrottlesCorrectly(t *testing.T) {
	// 10 req/s → one token per 100 ms.
	rl := provider.NewRateLimiter(provider.RateLimitConfig{
		RequestsPerSecond: 10,
		BurstSize:         1,
	})

	if !rl.Allow() {
		t.Fatal("Allow(): expected true on fresh limiter")
	}

	start := time.Now()
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	elapsed := time.Since(start)

	// Allow generous lower bound to absorb CI scheduling jitter.
	if elapsed < 50*time.Millisecond {
		t.Errorf("Wait returned too quickly: %v (expected ≥50ms)", elapsed)
	}
}

// TestRateLimiter_Wait_ContextCancel verifies that Wait returns ctx.Err() when
// the context is cancelled before a token becomes available.
func TestRateLimiter_Wait_ContextCancel(t *testing.T) {
	// 1 req/s → long wait, giving us plenty of time to cancel.
	rl := provider.NewRateLimiter(provider.RateLimitConfig{
		RequestsPerSecond: 1,
		BurstSize:         1,
	})

	if !rl.Allow() {
		t.Fatal("Allow(): expected true on fresh limiter")
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- rl.Wait(ctx)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-result:
		if err == nil {
			t.Error("Wait: expected error after context cancellation, got nil")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Wait did not return after context cancellation")
	}
}
