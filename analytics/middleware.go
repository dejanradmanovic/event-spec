package analytics

import "net/http"

// AnalyticsMiddleware injects a TransactionContext into each request's context.Context
// using WithAnalyticsContext. The injected context is available in handlers via
// TransactionContextFrom, and the analytics Client picks it up automatically
// as level-2 context in the 4-level precedence chain.
//
// Usage:
//
//	mux.Handle("/", AnalyticsMiddleware(extractFunc)(handler))
//
// where extractFunc returns the TransactionContext for the request.
func AnalyticsMiddleware(extract func(r *http.Request) TransactionContext) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			txCtx := extract(r)
			ctx := WithAnalyticsContext(r.Context(), txCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
