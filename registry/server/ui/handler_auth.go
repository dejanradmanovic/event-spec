package ui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"net/http"
	"strings"

	"github.com/dejanradmanovic/event-spec/registry/server/shared"
)

func sha256hex(s string) string { return shared.Sha256Hex(s) }
func roleLevel(role string) int { return shared.RoleLevel(role) }

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
	count, err := h.st.CountAPIKeys(r.Context())
	if err == nil && count == 0 {
		http.Redirect(w, r, "/ui/bootstrap", http.StatusFound)
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

func (h *Handler) handleBootstrapForm(w http.ResponseWriter, r *http.Request) {
	count, err := h.st.CountAPIKeys(r.Context())
	if err != nil || count > 0 {
		if sessionKey(r) != "" {
			http.Redirect(w, r, "/ui/", http.StatusFound)
		} else {
			http.Redirect(w, r, "/ui/login", http.StatusFound)
		}
		return
	}
	h.renderBootstrap(w, "", "")
}

func (h *Handler) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	count, err := h.st.CountAPIKeys(r.Context())
	if err != nil || count > 0 {
		http.Redirect(w, r, "/ui/login", http.StatusFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderBootstrap(w, "Invalid request.", "")
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = "Initial admin key"
	}
	identity := strings.TrimSpace(r.FormValue("identity"))
	if identity == "" {
		identity = "bootstrap"
	}
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		h.renderBootstrap(w, "Failed to generate key. Please try again.", "")
		return
	}
	rawKey := hex.EncodeToString(rawBytes)
	if _, err := h.st.CreateAPIKey(r.Context(), sha256hex(rawKey), RoleAdmin, name, identity, nil); err != nil {
		h.renderBootstrap(w, "Failed to create key: "+err.Error(), "")
		return
	}
	setSessionCookie(w, rawKey)
	h.renderBootstrap(w, "", rawKey)
}

func (h *Handler) renderBootstrap(w http.ResponseWriter, flash, newKey string) {
	t, err := template.New("").ParseFS(FS, "templates/bootstrap.html")
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = t.ExecuteTemplate(w, "bootstrap", struct {
		Flash  string
		NewKey string
	}{flash, newKey})
}
