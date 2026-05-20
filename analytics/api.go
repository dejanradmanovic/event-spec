package analytics

import (
	"context"
	"fmt"
	"sync"

	"github.com/dejanradmanovic/event-spec/hooks"
	"github.com/dejanradmanovic/event-spec/provider"
)

var (
	globalMu        sync.RWMutex
	globalCtxValue  AnalyticsContext
	globalHooksList []hooks.Hook
	// globalClient is the default client used by package-level functions.
	globalClient = &Client{}
)

// getGlobalContext returns the current global AnalyticsContext (level 1).
// Called by Client.mergeContextChain for every dispatch.
func getGlobalContext() AnalyticsContext {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalCtxValue
}

// getGlobalHooks returns a snapshot of the current global hook list (api-hooks).
// Called by Client.collectAllHooks for every dispatch.
func getGlobalHooks() []hooks.Hook {
	globalMu.RLock()
	defer globalMu.RUnlock()
	h := make([]hooks.Hook, len(globalHooksList))
	copy(h, globalHooksList)
	return h
}

// SetGlobalProvider replaces all providers on the global client.
func SetGlobalProvider(p ...provider.Provider) error {
	globalClient.mu.Lock()
	globalClient.providers = p
	globalClient.mu.Unlock()
	return nil
}

// AddGlobalProvider appends a provider to the global client.
func AddGlobalProvider(p provider.Provider) error {
	globalClient.mu.Lock()
	globalClient.providers = append(globalClient.providers, p)
	globalClient.mu.Unlock()
	return nil
}

// SetGlobalContext sets the global-level AnalyticsContext (level 1, lowest priority).
// Applies to ALL clients, not just the global one.
func SetGlobalContext(ctx AnalyticsContext) {
	globalMu.Lock()
	globalCtxValue = ctx
	globalMu.Unlock()
}

// AddGlobalHooks appends hooks to the global api-hook chain.
// These run first in the governance-first order and apply to ALL clients.
func AddGlobalHooks(h ...hooks.Hook) {
	globalMu.Lock()
	globalHooksList = append(globalHooksList, h...)
	globalMu.Unlock()
}

// NewClient creates a new independent analytics Client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Track dispatches a track event using the global client.
func Track(ctx context.Context, event Event, opts ...TrackOption) error {
	return globalClient.Track(ctx, event, opts...)
}

// Shutdown flushes and shuts down all providers on the global client.
func Shutdown(ctx context.Context) error {
	globalClient.mu.RLock()
	ps := make([]provider.Provider, len(globalClient.providers))
	copy(ps, globalClient.providers)
	globalClient.mu.RUnlock()

	var errs []error
	for _, p := range ps {
		if err := p.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.Metadata().Name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("shutdown: %v", errs)
	}
	return nil
}
