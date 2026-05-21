package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]any{
		"status": "ok",
		"uptime": time.Since(s.startedAt).String(),
	})
}

func (s *Server) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := AuditFilter{Limit: 50}
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			filter.Limit = n
		}
	}
	if v := q.Get("entity"); v != "" {
		filter.EntityType = v
	}
	if v := q.Get("user"); v != "" {
		filter.UserID = v
	}
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.Since = &t
		}
	}
	if v := q.Get("until"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.Until = &t
		}
	}
	entries, err := s.st.ListAuditLog(r.Context(), filter)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []AuditEntry{}
	}
	jsonOK(w, entries)
}

func (s *Server) handleRegisterWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		jsonError(w, "url is required", http.StatusBadRequest)
		return
	}
	userID, _ := r.Context().Value(ctxUserID).(string)
	if err := s.st.RegisterWebhook(r.Context(), req.URL, userID); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleListWebhooksAdmin(w http.ResponseWriter, r *http.Request) {
	records, err := s.st.ListWebhooksAdmin(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if records == nil {
		records = []WebhookRecord{}
	}
	jsonOK(w, records)
}

func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid webhook id", http.StatusBadRequest)
		return
	}
	if err := s.st.DeleteWebhook(r.Context(), id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleCreateAPIKey creates a new API key. On a zero-key server (bootstrap), no auth is
// required. On a server with existing keys, an admin token is required.
func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	count, err := s.st.CountAPIKeys(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if count > 0 {
		token := extractBearer(r)
		if token == "" {
			jsonError(w, "missing Authorization: Bearer header", http.StatusUnauthorized)
			return
		}
		_, role, err := s.st.LookupAPIKey(r.Context(), sha256hex(token))
		if err != nil {
			jsonError(w, "invalid or expired API key", http.StatusUnauthorized)
			return
		}
		if roleLevel(role) < roleLevel(RoleAdmin) {
			jsonError(w, "forbidden: requires role admin", http.StatusForbidden)
			return
		}
	}

	var req struct {
		Role      string `json:"role"`
		Name      string `json:"name"`
		ExpiresIn string `json:"expires_in"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Role == "" {
		jsonError(w, "role is required", http.StatusBadRequest)
		return
	}
	if roleLevel(req.Role) < 0 {
		jsonError(w, "invalid role: expected viewer, publisher, or admin", http.StatusBadRequest)
		return
	}

	var expiresAt *time.Time
	if req.ExpiresIn != "" {
		d, err := parseExtendedDuration(req.ExpiresIn)
		if err != nil {
			jsonError(w, "invalid expires_in: "+err.Error(), http.StatusBadRequest)
			return
		}
		t := time.Now().Add(d)
		expiresAt = &t
	}

	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		jsonError(w, "generate key: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rawKey := hex.EncodeToString(rawBytes)
	keyHash := sha256hex(rawKey)

	userID, _ := r.Context().Value(ctxUserID).(string)
	if userID == "" {
		userID = "bootstrap"
	}

	id, err := s.st.CreateAPIKey(r.Context(), keyHash, req.Role, req.Name, userID, expiresAt)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck // response write errors are not actionable from a handler
		"id":   id,
		"key":  rawKey,
		"role": req.Role,
	})
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	records, err := s.st.ListAPIKeys(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if records == nil {
		records = []APIKeyRecord{}
	}
	jsonOK(w, records)
}

func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid key id", http.StatusBadRequest)
		return
	}
	if err := s.st.RevokeAPIKey(r.Context(), id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseExtendedDuration parses Go duration strings plus "Nd" (days) and "Ny" (years).
func parseExtendedDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	if strings.HasSuffix(s, "y") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "y"))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n) * 365 * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
