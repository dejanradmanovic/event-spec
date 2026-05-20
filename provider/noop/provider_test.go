package noop

import (
	"context"
	"testing"
	"time"

	"event-spec/provider"
)

// compile-time check: *Provider satisfies provider.Provider.
var _ provider.Provider = (*Provider)(nil)

func TestNew(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("New() returned nil")
	}
}

func TestMetadata(t *testing.T) {
	p := New()
	meta := p.Metadata()

	if meta.Name != "noop" {
		t.Errorf("Name = %q, want %q", meta.Name, "noop")
	}
	if meta.Version == "" {
		t.Error("Version is empty")
	}

	caps := meta.Capabilities
	if !caps.Track {
		t.Error("Capabilities.Track = false, want true")
	}
	if !caps.Identify {
		t.Error("Capabilities.Identify = false, want true")
	}
	if !caps.Group {
		t.Error("Capabilities.Group = false, want true")
	}
	if !caps.Page {
		t.Error("Capabilities.Page = false, want true")
	}
	if !caps.Alias {
		t.Error("Capabilities.Alias = false, want true")
	}
}

func TestHooks(t *testing.T) {
	p := New()
	if hooks := p.Hooks(); len(hooks) != 0 {
		t.Errorf("Hooks() len = %d, want 0", len(hooks))
	}
}

func TestTrack(t *testing.T) {
	p := New()
	err := p.Track(context.Background(), provider.TrackMessage{
		MessageID: "msg-1",
		Timestamp: time.Now(),
		EventName: "Test Event",
		UserID:    "user-1",
	})
	if err != nil {
		t.Errorf("Track() error = %v, want nil", err)
	}
}

func TestIdentify(t *testing.T) {
	p := New()
	err := p.Identify(context.Background(), provider.IdentifyMessage{
		MessageID: "msg-2",
		Timestamp: time.Now(),
		UserID:    "user-1",
		Traits:    map[string]any{"email": "user@example.com"},
	})
	if err != nil {
		t.Errorf("Identify() error = %v, want nil", err)
	}
}

func TestGroup(t *testing.T) {
	p := New()
	err := p.Group(context.Background(), provider.GroupMessage{
		MessageID: "msg-3",
		Timestamp: time.Now(),
		UserID:    "user-1",
		GroupID:   "group-1",
	})
	if err != nil {
		t.Errorf("Group() error = %v, want nil", err)
	}
}

func TestPage(t *testing.T) {
	p := New()
	err := p.Page(context.Background(), provider.PageMessage{
		MessageID: "msg-4",
		Timestamp: time.Now(),
		UserID:    "user-1",
		Name:      "Home",
	})
	if err != nil {
		t.Errorf("Page() error = %v, want nil", err)
	}
}

func TestAlias(t *testing.T) {
	p := New()
	err := p.Alias(context.Background(), provider.AliasMessage{
		MessageID:  "msg-5",
		Timestamp:  time.Now(),
		UserID:     "user-1",
		PreviousID: "anon-1",
	})
	if err != nil {
		t.Errorf("Alias() error = %v, want nil", err)
	}
}

func TestFlush(t *testing.T) {
	p := New()
	if err := p.Flush(context.Background()); err != nil {
		t.Errorf("Flush() error = %v, want nil", err)
	}
}

func TestShutdown(t *testing.T) {
	p := New()
	if err := p.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown() error = %v, want nil", err)
	}
}

func TestAllOperationsWithCancelledContext(t *testing.T) {
	p := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := p.Track(ctx, provider.TrackMessage{}); err != nil {
		t.Errorf("Track() with cancelled ctx error = %v, want nil", err)
	}
	if err := p.Identify(ctx, provider.IdentifyMessage{}); err != nil {
		t.Errorf("Identify() with cancelled ctx error = %v, want nil", err)
	}
	if err := p.Group(ctx, provider.GroupMessage{}); err != nil {
		t.Errorf("Group() with cancelled ctx error = %v, want nil", err)
	}
	if err := p.Page(ctx, provider.PageMessage{}); err != nil {
		t.Errorf("Page() with cancelled ctx error = %v, want nil", err)
	}
	if err := p.Alias(ctx, provider.AliasMessage{}); err != nil {
		t.Errorf("Alias() with cancelled ctx error = %v, want nil", err)
	}
	if err := p.Flush(ctx); err != nil {
		t.Errorf("Flush() with cancelled ctx error = %v, want nil", err)
	}
	if err := p.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() with cancelled ctx error = %v, want nil", err)
	}
}
