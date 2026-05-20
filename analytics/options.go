package analytics

import (
	"github.com/dejanradmanovic/event-spec/hooks"
	"github.com/dejanradmanovic/event-spec/provider"
)

// trackOptions holds resolved per-invocation overrides.
type trackOptions struct {
	contextOverride *AnalyticsContext
	hints           hooks.HookHints
}

// ClientOption configures a Client at construction time.
type ClientOption func(*Client)

// TrackOption configures a single Track/Identify/Group/Page/Alias call.
type TrackOption func(*trackOptions)

// WithProviders adds providers to a new Client.
func WithProviders(p ...provider.Provider) ClientOption {
	return func(c *Client) {
		c.providers = append(c.providers, p...)
	}
}

// WithContext sets the client-level AnalyticsContext on a new Client.
func WithContext(ac AnalyticsContext) ClientOption {
	return func(c *Client) {
		c.clientCtx = ac
	}
}

// WithHooks registers hooks on a new Client. They run after global api-hooks
// and before provider-hooks in the governance-first chain.
func WithHooks(h ...hooks.Hook) ClientOption {
	return func(c *Client) {
		c.hooks = append(c.hooks, h...)
	}
}

// WithContextOverride overrides the AnalyticsContext for a single invocation (level 4, highest).
func WithContextOverride(ac AnalyticsContext) TrackOption {
	return func(o *trackOptions) {
		o.contextOverride = &ac
	}
}

func resolveOptions(opts []TrackOption) *trackOptions {
	to := &trackOptions{}
	for _, opt := range opts {
		opt(to)
	}
	return to
}
