package server

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

func (s *Server) routes() {
	s.mux.HandleFunc("GET /v1/health", s.handleStatus)
	s.mux.HandleFunc("POST /v1/events", s.withAuth(RolePublisher, s.handlePublishEvent))
	s.mux.HandleFunc("GET /v1/events", s.withAuth(RoleViewer, s.handleListEvents))
	s.mux.HandleFunc("GET /v1/events/{namespace}/{name}", s.withAuth(RoleViewer, s.handleGetEvent))
	s.mux.HandleFunc("GET /v1/events/{namespace}/{name}/{version}", s.withAuth(RoleViewer, s.handleGetEventVersion))
	s.mux.HandleFunc("GET /v1/diff/{namespace}/{name}/{from}/{to}", s.withAuth(RoleViewer, s.handleDiff))
	s.mux.HandleFunc("GET /v1/sources/{name}/pull", s.withAuth(RoleViewer, s.handleSourcePull))
	s.mux.HandleFunc("GET /v1/audit", s.withAuth(RoleAdmin, s.handleAuditLog))
	s.mux.HandleFunc("POST /v1/webhooks", s.withAuth(RoleAdmin, s.handleRegisterWebhook))
	s.mux.HandleFunc("GET /v1/webhooks", s.withAuth(RoleAdmin, s.handleListWebhooksAdmin))
	s.mux.HandleFunc("DELETE /v1/webhooks/{id}", s.withAuth(RoleAdmin, s.handleDeleteWebhook))
	s.mux.HandleFunc("POST /v1/admin/keys", s.handleCreateAPIKey)
	s.mux.HandleFunc("GET /v1/admin/keys", s.withAuth(RoleAdmin, s.handleListAPIKeys))
	s.mux.HandleFunc("DELETE /v1/admin/keys/{id}", s.withAuth(RoleAdmin, s.handleRevokeAPIKey))
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

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := registry.ListFilter{
		Namespace: q.Get("namespace"),
		Status:    spec.EventStatus(q.Get("status")),
		Tags:      q["tag"],
	}
	events, err := s.st.ListEvents(r.Context(), filter)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if events == nil {
		events = []spec.EventDef{}
	}
	jsonOK(w, events)
}

func (s *Server) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	def, err := s.st.GetEvent(r.Context(), namespace, name, "")
	if err != nil {
		writeEventError(w, err)
		return
	}
	jsonOK(w, def)
}

func (s *Server) handleGetEventVersion(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	version := r.PathValue("version")
	def, err := s.st.GetEvent(r.Context(), namespace, name, version)
	if err != nil {
		writeEventError(w, err)
		return
	}
	jsonOK(w, def)
}

func (s *Server) handlePublishEvent(w http.ResponseWriter, r *http.Request) {
	var event spec.EventDef
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if event.Namespace == "" || event.Name == "" || event.Version == "" {
		jsonError(w, "namespace, name, and version are required", http.StatusBadRequest)
		return
	}
	if event.Status == "" {
		event.Status = spec.StatusDraft
	}
	userID, _ := r.Context().Value(ctxUserID).(string)
	if err := s.st.PublishEvent(r.Context(), event, userID); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	go s.fireWebhooks(event, userID)
	w.WriteHeader(http.StatusCreated)
}

// fireWebhooks dispatches an HTTP POST to every registered webhook URL.
// It runs in a goroutine so it never blocks the HTTP response.
func (s *Server) fireWebhooks(event spec.EventDef, publishedBy string) {
	urls, err := s.st.ListWebhooks(context.Background())
	if err != nil || len(urls) == 0 {
		return
	}
	payload, err := json.Marshal(WebhookPayload{Event: event, PublishedBy: publishedBy})
	if err != nil {
		return
	}
	for _, u := range urls {
		go func(u string) {
			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, u, bytes.NewReader(payload))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				_ = resp.Body.Close()
			}
		}(u)
	}
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	from := r.PathValue("from")
	to := r.PathValue("to")

	fromDef, err := s.st.GetEvent(r.Context(), namespace, name, from)
	if err != nil {
		writeEventError(w, err)
		return
	}
	toDef, err := s.st.GetEvent(r.Context(), namespace, name, to)
	if err != nil {
		writeEventError(w, err)
		return
	}
	changes := spec.Diff(fromDef, toDef)
	if changes == nil {
		changes = []spec.Change{}
	}
	jsonOK(w, changes)
}

func (s *Server) handleSourcePull(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	src, err := s.st.GetSource(r.Context(), name)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			jsonError(w, "source not found: "+name, http.StatusNotFound)
		} else {
			jsonError(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	all, err := s.st.ListEvents(r.Context(), registry.ListFilter{})
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, ev := range all {
		if !sourceIncludesEvent(src.Events, ev.Namespace, ev.Name) {
			continue
		}
		fileName := ev.Namespace + "/" + ev.Name + "/" + ev.Version + ".yaml"
		fw, err := zw.Create(fileName)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data, _ := yaml.Marshal(ev)
		if _, err := fw.Write(data); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if err := zw.Close(); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="specs.zip"`)
	w.Write(buf.Bytes()) //nolint:errcheck // response write errors are not actionable
}

// sourceIncludesEvent reports whether namespace/name matches any of the source's event patterns.
func sourceIncludesEvent(patterns []string, namespace, name string) bool {
	target := namespace + "/" + name
	for _, p := range patterns {
		if matchGlob(p, target) {
			return true
		}
	}
	return false
}

// matchGlob matches target against a simple glob pattern.
// Supports ** at the end of a path segment as a wildcard for any sub-path.
func matchGlob(pattern, target string) bool {
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return strings.HasPrefix(target, prefix+"/") || target == prefix
	}
	return pattern == target
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

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]any{
		"status": "ok",
		"uptime": time.Since(s.startedAt).String(),
	})
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

func writeEventError(w http.ResponseWriter, err error) {
	if errors.Is(err, registry.ErrNotFound) {
		jsonError(w, err.Error(), http.StatusNotFound)
	} else {
		jsonError(w, err.Error(), http.StatusInternalServerError)
	}
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck // response write errors are not actionable from a handler
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck // response write errors are not actionable from a handler
}
