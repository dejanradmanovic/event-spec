// Package noop provides a no-op analytics provider for use as a safe default
// and in tests where real provider delivery is not needed.
package noop

import (
	"context"

	"github.com/dejanradmanovic/event-spec/hooks"
	"github.com/dejanradmanovic/event-spec/provider"
)

// Provider is a no-op implementation of provider.Provider.
// All analytics methods return nil; Hooks returns an empty slice.
type Provider struct{}

// New returns a noop Provider that satisfies provider.Provider without
// delivering events to any destination.
func New() *Provider {
	return &Provider{}
}

// Metadata returns identifying information for the noop provider.
func (p *Provider) Metadata() provider.ProviderMetadata {
	return provider.ProviderMetadata{
		Name:    "noop",
		Version: "0.1.0",
		Capabilities: provider.ProviderCapabilities{
			Track:    true,
			Identify: true,
			Group:    true,
			Page:     true,
			Alias:    true,
		},
	}
}

// Hooks returns no provider-level hooks.
func (p *Provider) Hooks() []hooks.Hook { return nil }

// Track discards the event and returns nil.
func (p *Provider) Track(_ context.Context, _ provider.TrackMessage) error { return nil }

// Identify discards the call and returns nil.
func (p *Provider) Identify(_ context.Context, _ provider.IdentifyMessage) error { return nil }

// Group discards the call and returns nil.
func (p *Provider) Group(_ context.Context, _ provider.GroupMessage) error { return nil }

// Page discards the call and returns nil.
func (p *Provider) Page(_ context.Context, _ provider.PageMessage) error { return nil }

// Alias discards the call and returns nil.
func (p *Provider) Alias(_ context.Context, _ provider.AliasMessage) error { return nil }

// Flush is a no-op; there is nothing to flush.
func (p *Provider) Flush(_ context.Context) error { return nil }

// Shutdown is a no-op; there are no resources to release.
func (p *Provider) Shutdown(_ context.Context) error { return nil }
