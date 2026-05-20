// Package testutil provides test helpers for the event-spec runtime.
package testutil

import (
	"context"
	"sync"

	"github.com/dejanradmanovic/event-spec/hooks"
	"github.com/dejanradmanovic/event-spec/provider"
)

// CaptureProvider records every provider call for assertion in tests.
// It is safe for concurrent use.
type CaptureProvider struct {
	mu   sync.Mutex
	name string

	Tracks     []provider.TrackMessage
	Identifies []provider.IdentifyMessage
	Groups     []provider.GroupMessage
	Pages      []provider.PageMessage
	Aliases    []provider.AliasMessage

	// TrackErr, if non-nil, is returned by Track to simulate failures.
	TrackErr    error
	IdentifyErr error
	GroupErr    error
	PageErr     error
	AliasErr    error

	FlushCalls    int
	ShutdownCalls int
	NoopCalls     int
}

// NewCaptureProvider returns a CaptureProvider with the given name.
func NewCaptureProvider(name string) *CaptureProvider {
	return &CaptureProvider{name: name}
}

// Metadata returns provider metadata identifying this capture provider.
func (c *CaptureProvider) Metadata() provider.ProviderMetadata {
	return provider.ProviderMetadata{
		Name:    c.name,
		Version: "0.0.0-test",
		Capabilities: provider.ProviderCapabilities{
			Track:    true,
			Identify: true,
			Group:    true,
			Page:     true,
			Alias:    true,
		},
	}
}

// Hooks returns nil; CaptureProvider registers no hooks.
func (c *CaptureProvider) Hooks() []hooks.Hook { return nil }

// Track records the message and returns TrackErr.
func (c *CaptureProvider) Track(_ context.Context, msg provider.TrackMessage) error {
	c.mu.Lock()
	c.Tracks = append(c.Tracks, msg)
	c.mu.Unlock()
	return c.TrackErr
}

// Identify records the message and returns IdentifyErr.
func (c *CaptureProvider) Identify(_ context.Context, msg provider.IdentifyMessage) error {
	c.mu.Lock()
	c.Identifies = append(c.Identifies, msg)
	c.mu.Unlock()
	return c.IdentifyErr
}

// Group records the message and returns GroupErr.
func (c *CaptureProvider) Group(_ context.Context, msg provider.GroupMessage) error {
	c.mu.Lock()
	c.Groups = append(c.Groups, msg)
	c.mu.Unlock()
	return c.GroupErr
}

// Page records the message and returns PageErr.
func (c *CaptureProvider) Page(_ context.Context, msg provider.PageMessage) error {
	c.mu.Lock()
	c.Pages = append(c.Pages, msg)
	c.mu.Unlock()
	return c.PageErr
}

// Alias records the message and returns AliasErr.
func (c *CaptureProvider) Alias(_ context.Context, msg provider.AliasMessage) error {
	c.mu.Lock()
	c.Aliases = append(c.Aliases, msg)
	c.mu.Unlock()
	return c.AliasErr
}

// OnNoopCall increments NoopCalls; called by the runtime when an event is dropped via noop.
func (c *CaptureProvider) OnNoopCall() {
	c.mu.Lock()
	c.NoopCalls++
	c.mu.Unlock()
}

// Flush increments FlushCalls and returns nil.
func (c *CaptureProvider) Flush(_ context.Context) error {
	c.mu.Lock()
	c.FlushCalls++
	c.mu.Unlock()
	return nil
}

// Shutdown increments ShutdownCalls and returns nil.
func (c *CaptureProvider) Shutdown(_ context.Context) error {
	c.mu.Lock()
	c.ShutdownCalls++
	c.mu.Unlock()
	return nil
}

// Events returns a snapshot of all recorded TrackMessages.
func (c *CaptureProvider) Events() []provider.TrackMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]provider.TrackMessage, len(c.Tracks))
	copy(out, c.Tracks)
	return out
}

// Reset clears all recorded messages and resets call counters.
func (c *CaptureProvider) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Tracks = nil
	c.Identifies = nil
	c.Groups = nil
	c.Pages = nil
	c.Aliases = nil
	c.FlushCalls = 0
	c.ShutdownCalls = 0
	c.NoopCalls = 0
}
