package analytics

import (
	"context"

	"github.com/dejanradmanovic/event-spec/hooks"
)

// AnalyticsContext is a type alias for hooks.AnalyticsContext.
// Aliased here (not copied) so analytics.AnalyticsContext and hooks.AnalyticsContext
// are the same type — no conversion needed when building HookContext.
type AnalyticsContext = hooks.AnalyticsContext

// Merge combines base and override AnalyticsContexts.
// Non-empty override fields win. Attributes are merged key-by-key with override keys winning.
func Merge(base, override AnalyticsContext) AnalyticsContext {
	result := AnalyticsContext{
		UserID:      base.UserID,
		AnonymousID: base.AnonymousID,
	}
	if override.UserID != "" {
		result.UserID = override.UserID
	}
	if override.AnonymousID != "" {
		result.AnonymousID = override.AnonymousID
	}
	if len(base.Attributes) > 0 || len(override.Attributes) > 0 {
		result.Attributes = make(map[string]any, len(base.Attributes)+len(override.Attributes))
		for k, v := range base.Attributes {
			result.Attributes[k] = v
		}
		for k, v := range override.Attributes {
			result.Attributes[k] = v
		}
	}
	return result
}

type analyticsContextKey struct{}

// WithAnalyticsContext stores a TransactionContext in a stdlib context.Context.
// Retrieve it with TransactionContextFrom. This is the only analytics context level
// stored in context.Context — used by AnalyticsMiddleware for per-request scope.
func WithAnalyticsContext(ctx context.Context, tx TransactionContext) context.Context {
	return context.WithValue(ctx, analyticsContextKey{}, tx)
}

// TransactionContextFrom retrieves the TransactionContext stored by WithAnalyticsContext.
func TransactionContextFrom(ctx context.Context) (TransactionContext, bool) {
	tx, ok := ctx.Value(analyticsContextKey{}).(TransactionContext)
	return tx, ok
}
