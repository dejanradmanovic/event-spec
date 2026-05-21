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
