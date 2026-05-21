package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/dejanradmanovic/event-spec/registry/server/shared"
)

// Role constants — aliases to shared to keep existing server-package code unchanged.
const (
	RoleViewer    = shared.RoleViewer
	RolePublisher = shared.RolePublisher
	RoleAdmin     = shared.RoleAdmin
)

// contextKey is an unexported type for context values to avoid collisions.
type contextKey int

const (
	ctxUserID contextKey = iota
	ctxRole
)

func sha256hex(s string) string { return shared.Sha256Hex(s) }
func roleLevel(role string) int { return shared.RoleLevel(role) }

// withAuth wraps a handler with Bearer token authentication and role enforcement.
// Authenticated identity is stored in the request context for downstream handlers.
func (s *Server) withAuth(minRole string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractBearer(r)
		if token == "" {
			jsonError(w, "missing Authorization: Bearer header", http.StatusUnauthorized)
			return
		}
		userID, role, err := s.st.LookupAPIKey(r.Context(), sha256hex(token))
		if err != nil {
			jsonError(w, "invalid or expired API key", http.StatusUnauthorized)
			return
		}
		if roleLevel(role) < roleLevel(minRole) {
			jsonError(w, "forbidden: requires role "+minRole, http.StatusForbidden)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID, userID)
		ctx = context.WithValue(ctx, ctxRole, role)
		next(w, r.WithContext(ctx))
	}
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimPrefix(h, prefix)
}
