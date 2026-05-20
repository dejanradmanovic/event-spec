package analytics

// TransactionContext is the per-request scope stored in context.Context via
// WithAnalyticsContext. It is the only analytics context level that uses the
// stdlib context.Context for storage, enabling HTTP middleware injection.
type TransactionContext struct {
	UserID      string
	AnonymousID string
	Attributes  map[string]any
}
