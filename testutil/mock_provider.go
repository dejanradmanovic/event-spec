package testutil

import (
	"context"
	"time"

	"event-spec/hooks"
	"event-spec/provider"
)

// MockProvider is a configurable test double that returns preset errors and/or
// simulates latency on each provider operation. It does not record events;
// pair it with CaptureProvider when recording is also needed.
type MockProvider struct {
	name string

	// Latency, if non-zero, is the artificial delay applied before each operation returns.
	Latency time.Duration

	// Per-operation errors returned after the simulated latency.
	TrackErr    error
	IdentifyErr error
	GroupErr    error
	PageErr     error
	AliasErr    error
	FlushErr    error
	ShutdownErr error
}

// NewMockProvider returns a MockProvider with the given name.
func NewMockProvider(name string) *MockProvider {
	return &MockProvider{name: name}
}

func (m *MockProvider) delay(ctx context.Context) error {
	if m.Latency <= 0 {
		return nil
	}
	select {
	case <-time.After(m.Latency):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Metadata returns provider metadata identifying this mock provider.
func (m *MockProvider) Metadata() provider.ProviderMetadata {
	return provider.ProviderMetadata{
		Name:    m.name,
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

// Hooks returns nil; MockProvider registers no hooks.
func (m *MockProvider) Hooks() []hooks.Hook { return nil }

// Track sleeps for Latency then returns TrackErr.
func (m *MockProvider) Track(ctx context.Context, _ provider.TrackMessage) error {
	if err := m.delay(ctx); err != nil {
		return err
	}
	return m.TrackErr
}

// Identify sleeps for Latency then returns IdentifyErr.
func (m *MockProvider) Identify(ctx context.Context, _ provider.IdentifyMessage) error {
	if err := m.delay(ctx); err != nil {
		return err
	}
	return m.IdentifyErr
}

// Group sleeps for Latency then returns GroupErr.
func (m *MockProvider) Group(ctx context.Context, _ provider.GroupMessage) error {
	if err := m.delay(ctx); err != nil {
		return err
	}
	return m.GroupErr
}

// Page sleeps for Latency then returns PageErr.
func (m *MockProvider) Page(ctx context.Context, _ provider.PageMessage) error {
	if err := m.delay(ctx); err != nil {
		return err
	}
	return m.PageErr
}

// Alias sleeps for Latency then returns AliasErr.
func (m *MockProvider) Alias(ctx context.Context, _ provider.AliasMessage) error {
	if err := m.delay(ctx); err != nil {
		return err
	}
	return m.AliasErr
}

// Flush sleeps for Latency then returns FlushErr.
func (m *MockProvider) Flush(ctx context.Context) error {
	if err := m.delay(ctx); err != nil {
		return err
	}
	return m.FlushErr
}

// Shutdown sleeps for Latency then returns ShutdownErr.
func (m *MockProvider) Shutdown(ctx context.Context) error {
	if err := m.delay(ctx); err != nil {
		return err
	}
	return m.ShutdownErr
}
