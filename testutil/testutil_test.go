package testutil_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dejanradmanovic/event-spec/provider"
	"github.com/dejanradmanovic/event-spec/testutil"
)

// --- CaptureProvider ---

func TestCaptureProvider_RecordsTrack(t *testing.T) {
	cap := testutil.NewCaptureProvider("test")
	msg := provider.TrackMessage{EventName: "page_viewed", UserID: "u1"}
	_ = cap.Track(context.Background(), msg)

	evts := cap.Events()
	if len(evts) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evts))
	}
	if evts[0].EventName != "page_viewed" {
		t.Errorf("unexpected event name %q", evts[0].EventName)
	}
	if evts[0].UserID != "u1" {
		t.Errorf("unexpected user_id %q", evts[0].UserID)
	}
}

func TestCaptureProvider_RecordsAllOps(t *testing.T) {
	cap := testutil.NewCaptureProvider("test")
	ctx := context.Background()
	_ = cap.Track(ctx, provider.TrackMessage{})
	_ = cap.Identify(ctx, provider.IdentifyMessage{})
	_ = cap.Group(ctx, provider.GroupMessage{})
	_ = cap.Page(ctx, provider.PageMessage{})
	_ = cap.Alias(ctx, provider.AliasMessage{})
	_ = cap.Flush(ctx)
	_ = cap.Shutdown(ctx)

	if len(cap.Tracks) != 1 {
		t.Errorf("expected 1 Track, got %d", len(cap.Tracks))
	}
	if len(cap.Identifies) != 1 {
		t.Errorf("expected 1 Identify, got %d", len(cap.Identifies))
	}
	if len(cap.Groups) != 1 {
		t.Errorf("expected 1 Group, got %d", len(cap.Groups))
	}
	if len(cap.Pages) != 1 {
		t.Errorf("expected 1 Page, got %d", len(cap.Pages))
	}
	if len(cap.Aliases) != 1 {
		t.Errorf("expected 1 Alias, got %d", len(cap.Aliases))
	}
	if cap.FlushCalls != 1 {
		t.Errorf("expected 1 Flush, got %d", cap.FlushCalls)
	}
	if cap.ShutdownCalls != 1 {
		t.Errorf("expected 1 Shutdown, got %d", cap.ShutdownCalls)
	}
}

func TestCaptureProvider_Reset(t *testing.T) {
	cap := testutil.NewCaptureProvider("test")
	ctx := context.Background()
	_ = cap.Track(ctx, provider.TrackMessage{EventName: "a"})
	_ = cap.Track(ctx, provider.TrackMessage{EventName: "b"})
	_ = cap.Flush(ctx)

	cap.Reset()

	if len(cap.Events()) != 0 {
		t.Errorf("expected empty Events after Reset, got %d", len(cap.Events()))
	}
	if cap.FlushCalls != 0 {
		t.Errorf("expected FlushCalls=0 after Reset, got %d", cap.FlushCalls)
	}
}

func TestCaptureProvider_EventsReturnsCopy(t *testing.T) {
	cap := testutil.NewCaptureProvider("test")
	_ = cap.Track(context.Background(), provider.TrackMessage{EventName: "x"})
	evts := cap.Events()
	evts[0].EventName = "mutated"

	fresh := cap.Events()
	if fresh[0].EventName == "mutated" {
		t.Error("Events() should return a copy, not a shared slice")
	}
}

func TestCaptureProvider_ReturnsConfiguredErrors(t *testing.T) {
	want := errors.New("track failed")
	cap := testutil.NewCaptureProvider("test")
	cap.TrackErr = want
	got := cap.Track(context.Background(), provider.TrackMessage{})
	if !errors.Is(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestCaptureProvider_SatisfiesInterface(t *testing.T) {
	var _ provider.Provider = testutil.NewCaptureProvider("test")
}

// --- MockProvider ---

func TestMockProvider_ReturnsConfiguredError(t *testing.T) {
	want := errors.New("boom")
	m := testutil.NewMockProvider("test")
	m.TrackErr = want
	got := m.Track(context.Background(), provider.TrackMessage{})
	if !errors.Is(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestMockProvider_SimulatesLatency(t *testing.T) {
	m := testutil.NewMockProvider("test")
	m.Latency = 50 * time.Millisecond
	start := time.Now()
	_ = m.Track(context.Background(), provider.TrackMessage{})
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Errorf("expected latency >= 40ms, got %v", elapsed)
	}
}

func TestMockProvider_RespectsContextCancellation(t *testing.T) {
	m := testutil.NewMockProvider("test")
	m.Latency = 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := m.Track(ctx, provider.TrackMessage{})
	if err == nil {
		t.Error("expected error on context cancellation, got nil")
	}
}

func TestMockProvider_SatisfiesInterface(t *testing.T) {
	var _ provider.Provider = testutil.NewMockProvider("test")
}
