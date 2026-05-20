package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

func (s *Server) routes() {
	s.mux.HandleFunc("POST /v1/events", s.withAuth(RolePublisher, s.handlePublishEvent))
	s.mux.HandleFunc("GET /v1/events", s.withAuth(RoleViewer, s.handleListEvents))
	s.mux.HandleFunc("GET /v1/events/{namespace}/{name}", s.withAuth(RoleViewer, s.handleGetEvent))
	s.mux.HandleFunc("GET /v1/events/{namespace}/{name}/{version}", s.withAuth(RoleViewer, s.handleGetEventVersion))
	s.mux.HandleFunc("GET /v1/diff/{namespace}/{name}/{from}/{to}", s.withAuth(RoleViewer, s.handleDiff))
	s.mux.HandleFunc("GET /v1/sources/{name}/pull", s.withAuth(RoleViewer, s.handleSourcePull))
	s.mux.HandleFunc("GET /v1/audit", s.withAuth(RoleAdmin, s.handleAuditLog))
	s.mux.HandleFunc("POST /v1/webhooks", s.withAuth(RoleAdmin, s.handleRegisterWebhook))
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
	w.WriteHeader(http.StatusCreated)
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
	entries, err := s.st.ListAuditLog(r.Context())
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
