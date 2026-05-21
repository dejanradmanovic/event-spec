package ui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// --- audit log ---

type auditData struct {
	baseData
	Entries    []AuditEntry
	Page       int
	EntityType string
	UserFilter string
}

func (h *Handler) handleAudit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	entityType := q.Get("entity")
	userFilter := q.Get("user")

	filter := AuditFilter{
		Limit:      50,
		EntityType: entityType,
		UserID:     userFilter,
	}
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			filter.Since = &t
		}
	}

	entries, err := h.st.ListAuditLog(r.Context(), filter)
	if err != nil {
		h.renderErrorPage(w, r, http.StatusInternalServerError, "Audit Log", err.Error())
		return
	}

	b := newBase(r, "Audit Log")
	h.render(w, "audit", auditData{b, entries, page, entityType, userFilter})
}

// --- API keys ---

type keysData struct {
	baseData
	Keys     []APIKeyRecord
	NewKey   string
	NewKeyID int64
}

func (h *Handler) handleKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.st.ListAPIKeys(r.Context())
	if err != nil {
		h.renderErrorPage(w, r, http.StatusInternalServerError, "API Keys", err.Error())
		return
	}
	b := newBase(r, "API Keys")
	h.render(w, "keys", keysData{baseData: b, Keys: keys})
}

func (h *Handler) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/ui/settings/keys", http.StatusFound)
		return
	}
	role := r.FormValue("role")
	name := r.FormValue("name")
	identity := strings.TrimSpace(r.FormValue("identity"))
	expiresIn := r.FormValue("expires_in")

	if role == "" || roleLevel(role) < 0 {
		role = RoleViewer
	}

	var expiresAt *time.Time
	if expiresIn != "" {
		if d, err := parseSimpleDuration(expiresIn); err == nil {
			t := time.Now().Add(d)
			expiresAt = &t
		}
	}

	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		http.Redirect(w, r, "/ui/settings/keys", http.StatusFound)
		return
	}
	rawKey := hex.EncodeToString(rawBytes)
	keyHash := sha256hex(rawKey)

	sessionUser, _ := r.Context().Value(ctxUserID).(string)
	if identity == "" {
		identity = sessionUser
	}
	id, err := h.st.CreateAPIKey(r.Context(), keyHash, role, name, identity, expiresAt)
	if err != nil {
		http.Redirect(w, r, "/ui/settings/keys", http.StatusFound)
		return
	}

	keys, _ := h.st.ListAPIKeys(r.Context())
	b := newBase(r, "API Keys")
	b.Flash = "Key created successfully. Copy it now — it won't be shown again."

	if r.Header.Get("HX-Request") == "true" {
		h.renderPartial(w, "keys", "new-key-modal", struct {
			NewKey   string
			NewKeyID int64
		}{rawKey, id})
		return
	}
	h.render(w, "keys", keysData{b, keys, rawKey, id})
}

func (h *Handler) handleRevokeKey(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	_ = h.st.RevokeAPIKey(r.Context(), id)

	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/ui/settings/keys", http.StatusFound)
}

// --- webhooks ---

type webhooksData struct {
	baseData
	Webhooks []WebhookRecord
}

func (h *Handler) handleWebhooks(w http.ResponseWriter, r *http.Request) {
	webhooks, err := h.st.ListWebhooksAdmin(r.Context())
	if err != nil {
		h.renderErrorPage(w, r, http.StatusInternalServerError, "Webhooks", err.Error())
		return
	}
	b := newBase(r, "Webhooks")
	h.render(w, "webhooks", webhooksData{b, webhooks})
}

func (h *Handler) handleAddWebhook(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/ui/settings/webhooks", http.StatusFound)
		return
	}
	webhookURL := strings.TrimSpace(r.FormValue("url"))
	if webhookURL == "" {
		http.Redirect(w, r, "/ui/settings/webhooks", http.StatusFound)
		return
	}
	userID, _ := r.Context().Value(ctxUserID).(string)
	_ = h.st.RegisterWebhook(r.Context(), webhookURL, userID)
	http.Redirect(w, r, "/ui/settings/webhooks", http.StatusFound)
}

func (h *Handler) handleRemoveWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	_ = h.st.DeleteWebhook(r.Context(), id)

	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/ui/settings/webhooks", http.StatusFound)
}

// --- config ---

type configData struct {
	baseData
	Settings []ServerSetting
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	settings, err := h.st.ListSettings(r.Context())
	if err != nil {
		h.renderErrorPage(w, r, http.StatusInternalServerError, "Config", err.Error())
		return
	}
	// Ensure hooks_enabled always appears even if the DB row is absent.
	found := false
	for _, s := range settings {
		if s.Key == "hooks_enabled" {
			found = true
			break
		}
	}
	if !found {
		v := "true"
		if h.hooksState != nil && !h.hooksState() {
			v = "false"
		}
		settings = append([]ServerSetting{{Key: "hooks_enabled", Value: v}}, settings...)
	}
	b := newBase(r, "Server Config")
	h.render(w, "config", configData{b, settings})
}

func (h *Handler) handleSetConfig(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		if err2 := r.ParseForm(); err2 == nil {
			body.Value = r.FormValue("value")
		}
	}

	if key == "hooks_enabled" && body.Value != "true" && body.Value != "false" {
		http.Error(w, "value must be true or false", http.StatusBadRequest)
		return
	}

	if err := h.st.SetSetting(r.Context(), key, body.Value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = template.Must(template.New("").Parse(
			`<span class="badge bg-{{if eq .Value "true"}}green{{else}}red{{end}}">{{.Value}}</span>`,
		)).Execute(w, body)
		return
	}
	http.Redirect(w, r, "/ui/settings/config", http.StatusFound)
}
