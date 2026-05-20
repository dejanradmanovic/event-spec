package analytics

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"event-spec/hooks"
	"event-spec/provider"
)

// Client is the primary API surface for sending analytics events.
// Create one with NewClient; use the package-level functions for the global singleton.
type Client struct {
	mu        sync.RWMutex
	providers []provider.Provider
	hooks     []hooks.Hook        // client-level hooks (run after global api-hooks)
	clientCtx AnalyticsContext    // level-3 context
	txCtx     *TransactionContext // level-2 context (set by WithTransaction)
}

// AddProvider appends providers to this client's provider list.
func (c *Client) AddProvider(p ...provider.Provider) {
	c.mu.Lock()
	c.providers = append(c.providers, p...)
	c.mu.Unlock()
}

// SetContext updates the client-level AnalyticsContext (level 3).
func (c *Client) SetContext(ctx AnalyticsContext) {
	c.mu.Lock()
	c.clientCtx = ctx
	c.mu.Unlock()
}

// WithTransaction returns a shallow-cloned Client with txCtx stored at level 2.
// Use this when you need to bind a per-request identity without HTTP middleware.
func (c *Client) WithTransaction(txCtx TransactionContext) *Client {
	c.mu.RLock()
	clone := &Client{
		providers: c.providers,
		hooks:     c.hooks,
		clientCtx: c.clientCtx,
		txCtx:     &txCtx,
	}
	c.mu.RUnlock()
	return clone
}

// Track dispatches a track event to all configured providers.
// Returns a non-nil error only for pre-dispatch failures (hook cancelled the event).
// Post-dispatch per-provider failures are accessible via TrackDetailed.
func (c *Client) Track(ctx context.Context, event Event, opts ...TrackOption) error {
	_, err := c.TrackDetailed(ctx, event, opts...)
	return err
}

// TrackDetailed dispatches a track event and returns full per-provider outcomes.
func (c *Client) TrackDetailed(ctx context.Context, event Event, opts ...TrackOption) (DispatchResult, error) {
	return c.dispatchAll(ctx, "track", event.Name, event.Properties,
		func(ctx context.Context, p provider.Provider, msgID string, ts time.Time, env *hooks.EventEnvelope) error {
			return p.Track(ctx, provider.TrackMessage{
				MessageID:   msgID,
				Timestamp:   ts,
				EventName:   env.EventName,
				Properties:  env.Properties,
				UserID:      env.Context.UserID,
				AnonymousID: env.Context.AnonymousID,
			})
		}, opts)
}

// Identify sends an identify call to all configured providers.
func (c *Client) Identify(ctx context.Context, userID string, traits map[string]any, opts ...TrackOption) error {
	_, err := c.dispatchAll(ctx, "identify", "$identify", traits,
		func(ctx context.Context, p provider.Provider, msgID string, ts time.Time, env *hooks.EventEnvelope) error {
			uid := userID
			if env.Context.UserID != "" {
				uid = env.Context.UserID
			}
			return p.Identify(ctx, provider.IdentifyMessage{
				MessageID:   msgID,
				Timestamp:   ts,
				UserID:      uid,
				AnonymousID: env.Context.AnonymousID,
				Traits:      env.Properties,
			})
		}, opts)
	return err
}

// Group sends a group call to all configured providers.
func (c *Client) Group(ctx context.Context, groupID string, traits map[string]any, opts ...TrackOption) error {
	props := make(map[string]any, len(traits)+1)
	for k, v := range traits {
		props[k] = v
	}
	props["group_id"] = groupID
	_, err := c.dispatchAll(ctx, "group", "$group", props,
		func(ctx context.Context, p provider.Provider, msgID string, ts time.Time, env *hooks.EventEnvelope) error {
			gid := groupID
			if v, ok := env.Properties["group_id"]; ok {
				gid = fmt.Sprintf("%v", v)
			}
			return p.Group(ctx, provider.GroupMessage{
				MessageID:   msgID,
				Timestamp:   ts,
				UserID:      env.Context.UserID,
				AnonymousID: env.Context.AnonymousID,
				GroupID:     gid,
				Traits:      env.Properties,
			})
		}, opts)
	return err
}

// Page sends a page call to all configured providers.
func (c *Client) Page(ctx context.Context, name string, props map[string]any, opts ...TrackOption) error {
	p := make(map[string]any, len(props)+1)
	for k, v := range props {
		p[k] = v
	}
	p["name"] = name
	_, err := c.dispatchAll(ctx, "page", "$page", p,
		func(ctx context.Context, prov provider.Provider, msgID string, ts time.Time, env *hooks.EventEnvelope) error {
			pname := name
			if v, ok := env.Properties["name"]; ok {
				pname = fmt.Sprintf("%v", v)
			}
			return prov.Page(ctx, provider.PageMessage{
				MessageID:   msgID,
				Timestamp:   ts,
				UserID:      env.Context.UserID,
				AnonymousID: env.Context.AnonymousID,
				Name:        pname,
				Properties:  env.Properties,
			})
		}, opts)
	return err
}

// Alias sends an alias call to all configured providers.
func (c *Client) Alias(ctx context.Context, userID, previousID string, opts ...TrackOption) error {
	_, err := c.dispatchAll(ctx, "alias", "$alias", nil,
		func(ctx context.Context, p provider.Provider, msgID string, ts time.Time, env *hooks.EventEnvelope) error {
			return p.Alias(ctx, provider.AliasMessage{
				MessageID:  msgID,
				Timestamp:  ts,
				UserID:     userID,
				PreviousID: previousID,
			})
		}, opts)
	return err
}

// Flush flushes all configured providers.
func (c *Client) Flush(ctx context.Context) error {
	c.mu.RLock()
	ps := make([]provider.Provider, len(c.providers))
	copy(ps, c.providers)
	c.mu.RUnlock()

	var errs []error
	for _, p := range ps {
		if err := p.Flush(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.Metadata().Name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("flush: %v", errs)
	}
	return nil
}

// providerFn is the per-provider call executed concurrently during dispatch.
type providerFn func(ctx context.Context, p provider.Provider, msgID string, ts time.Time, env *hooks.EventEnvelope) error

// dispatchAll is the core event-processing pipeline shared by Track, Identify, Group, Page, Alias.
//
// Processing order:
//  1. Merge 4-level AnalyticsContext chain
//  2. Run all Before hooks (api → client → provider, governance-first)
//  3. Dispatch concurrently to each provider via fn
//  4. Run After/Error/Finally per provider in reverse hook order
//  5. Aggregate into DispatchResult
func (c *Client) dispatchAll(ctx context.Context, operation, eventName string, properties map[string]any, fn providerFn, opts []TrackOption) (DispatchResult, error) {
	to := resolveOptions(opts)
	merged := c.mergeContextChain(ctx, to.contextOverride)
	allHooks := c.collectAllHooks()

	hc := hooks.HookContext{
		Operation: operation,
		EventName: eventName,
		Context:   merged,
	}

	// Run Before hooks — runs once, gates ALL providers.
	env := &hooks.EventEnvelope{
		EventName:  eventName,
		Properties: cloneProperties(properties),
		Context:    merged,
	}
	for _, h := range allHooks {
		result, err := h.Before(ctx, hc, to.hints)
		if err != nil {
			return DispatchResult{}, err // hook cancelled the event
		}
		if result != nil {
			env = result
			hc.EventName = result.EventName
			hc.Context = result.Context
		}
	}

	c.mu.RLock()
	ps := make([]provider.Provider, len(c.providers))
	copy(ps, c.providers)
	c.mu.RUnlock()

	if len(ps) == 0 {
		return DispatchResult{}, nil
	}

	// Concurrent per-provider dispatch.
	results := make([]ProviderResult, len(ps))
	var wg sync.WaitGroup

	for i, p := range ps {
		wg.Add(1)
		go func(i int, p provider.Provider) {
			defer wg.Done()

			start := time.Now()
			callErr := fn(ctx, p, generateMessageID(), time.Now(), env)
			latency := time.Since(start)

			pName := p.Metadata().Name
			state := StateDelivered
			if callErr != nil {
				state = StateFailed
			}
			results[i] = ProviderResult{
				ProviderName: pName,
				State:        state,
				Error:        callErr,
				Latency:      latency,
			}

			// After/Error/Finally fire per provider in reverse hook order.
			provHC := hc
			provHC.Provider = pName
			hookResult := hooks.HookResult{Delivered: callErr == nil, Error: callErr}

			if callErr != nil {
				for j := len(allHooks) - 1; j >= 0; j-- {
					allHooks[j].Error(ctx, provHC, callErr, to.hints)
				}
			} else {
				for j := len(allHooks) - 1; j >= 0; j-- {
					_ = allHooks[j].After(ctx, provHC, hookResult, to.hints)
				}
			}
			for j := len(allHooks) - 1; j >= 0; j-- {
				allHooks[j].Finally(ctx, provHC, hookResult, to.hints)
			}
		}(i, p)
	}
	wg.Wait()

	var success, failed []ProviderResult
	for _, r := range results {
		if r.Error == nil {
			success = append(success, r)
		} else {
			failed = append(failed, r)
		}
	}
	return DispatchResult{
		Success:        success,
		Failed:         failed,
		PartialSuccess: len(success) > 0,
	}, nil
}

// mergeContextChain applies the 4-level precedence chain:
// global (1) → transaction/context.Context (2) → WithTransaction (2b) → client (3) → invocation (4).
func (c *Client) mergeContextChain(ctx context.Context, invocationOverride *AnalyticsContext) AnalyticsContext {
	result := getGlobalContext() // level 1

	// Level 2a: transaction from context.Context (HTTP middleware path).
	if tx, ok := TransactionContextFrom(ctx); ok {
		result = Merge(result, AnalyticsContext{
			UserID:      tx.UserID,
			AnonymousID: tx.AnonymousID,
			Attributes:  tx.Attributes,
		})
	}

	// Level 2b: transaction from WithTransaction (explicit path).
	c.mu.RLock()
	txCtx := c.txCtx
	clientCtx := c.clientCtx
	c.mu.RUnlock()

	if txCtx != nil {
		result = Merge(result, AnalyticsContext{
			UserID:      txCtx.UserID,
			AnonymousID: txCtx.AnonymousID,
			Attributes:  txCtx.Attributes,
		})
	}

	result = Merge(result, clientCtx) // level 3

	if invocationOverride != nil {
		result = Merge(result, *invocationOverride) // level 4
	}
	return result
}

// collectAllHooks returns api-hooks + client-hooks + provider-hooks in governance-first order.
func (c *Client) collectAllHooks() []hooks.Hook {
	apiHooks := getGlobalHooks()

	c.mu.RLock()
	clientHooks := make([]hooks.Hook, len(c.hooks))
	copy(clientHooks, c.hooks)
	ps := c.providers
	c.mu.RUnlock()

	var provHooks []hooks.Hook
	for _, p := range ps {
		provHooks = append(provHooks, p.Hooks()...)
	}

	all := make([]hooks.Hook, 0, len(apiHooks)+len(clientHooks)+len(provHooks))
	all = append(all, apiHooks...)
	all = append(all, clientHooks...)
	all = append(all, provHooks...)
	return all
}

// generateMessageID returns a UUID v4 string.
func generateMessageID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// cloneProperties returns a shallow copy of properties, or nil when src is nil.
func cloneProperties(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
