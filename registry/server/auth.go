package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// Role constants for API key authorization.
const (
	RoleViewer    = "viewer"
	RolePublisher = "publisher"
	RoleAdmin     = "admin"
)

// contextKey is an unexported type for context values to avoid collisions.
type contextKey int

const (
	ctxUserID contextKey = iota
	ctxRole
)

func sha256hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func roleLevel(role string) int {
	switch role {
	case RoleAdmin:
		return 2
	case RolePublisher:
		return 1
	default: // viewer or unknown
		return 0
	}
}

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
