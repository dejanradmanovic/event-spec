package ui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
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
	default:
		return 0
	}
}

func sessionKey(r *http.Request) string {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func setSessionCookie(w http.ResponseWriter, key string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    key,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 7,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1})
}

func (h *Handler) withSession(minRole string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := sessionKey(r)
		if key == "" {
			http.Redirect(w, r, "/ui/login", http.StatusFound)
			return
		}
		userID, role, err := h.st.LookupAPIKey(r.Context(), sha256hex(key))
		if err != nil {
			clearSessionCookie(w)
			http.Redirect(w, r, "/ui/login", http.StatusFound)
			return
		}
		if roleLevel(role) < roleLevel(minRole) {
			h.renderErrorPage(w, r, http.StatusForbidden, "Access denied", "This page requires the "+minRole+" role.")
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID, userID)
		ctx = context.WithValue(ctx, ctxRole, role)
		next(w, r.WithContext(ctx))
	}
}

func (h *Handler) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if sessionKey(r) != "" {
		http.Redirect(w, r, "/ui/", http.StatusFound)
		return
	}
	h.renderLogin(w, "", false)
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderLogin(w, "Invalid request.", true)
		return
	}
	key := strings.TrimSpace(r.FormValue("api_key"))
	if key == "" {
		h.renderLogin(w, "API key is required.", true)
		return
	}
	if _, _, err := h.st.LookupAPIKey(r.Context(), sha256hex(key)); err != nil {
		h.renderLogin(w, "Invalid or expired API key.", true)
		return
	}
	setSessionCookie(w, key)
	http.Redirect(w, r, "/ui/", http.StatusFound)
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w)
	http.Redirect(w, r, "/ui/login", http.StatusFound)
}
