package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/dejanradmanovic/event-spec/provider"
	"github.com/dejanradmanovic/event-spec/registry/server/ui"
	"github.com/dejanradmanovic/event-spec/spec"
)

func (s *Server) routes() {
	uiHandler := ui.New(
		s.st,
		func() bool { return s.hooksEnabled.Load() },
		func() time.Duration { return time.Since(s.startedAt) },
		s.uiPinger,
	)
	s.mux.Handle("/ui/", uiHandler)

	// Redirect bare root to the admin UI. Using "/" (no method prefix) registers
	// the ServeMux catch-all, which handles any path not matched by a more specific
	// pattern. Method-qualified patterns like "GET /v1/..." still take precedence.
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/ui/", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})
	s.mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/status", http.StatusFound)
	})

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
	s.mux.HandleFunc("GET /v1/admin/config", s.withAuth(RoleAdmin, s.handleGetConfig))
	s.mux.HandleFunc("PUT /v1/admin/config/{key}", s.withAuth(RoleAdmin, s.handleSetConfig))
	s.mux.HandleFunc("GET /v1/admin/sources", s.withAuth(RoleAdmin, s.handleListSources))
	s.mux.HandleFunc("POST /v1/admin/sources", s.withAuth(RoleAdmin, s.handleCreateSource))
	s.mux.HandleFunc("GET /v1/admin/sources/{name}", s.withAuth(RoleAdmin, s.handleGetSourceAdmin))
	s.mux.HandleFunc("PUT /v1/admin/sources/{name}", s.withAuth(RoleAdmin, s.handleUpdateSource))
	s.mux.HandleFunc("DELETE /v1/admin/sources/{name}", s.withAuth(RoleAdmin, s.handleDeleteSource))
	s.mux.HandleFunc("GET /v1/admin/destinations", s.withAuth(RoleAdmin, s.handleListDestinations))
	s.mux.HandleFunc("POST /v1/admin/destinations", s.withAuth(RoleAdmin, s.handleCreateDestination))
	s.mux.HandleFunc("GET /v1/admin/destinations/{name}", s.withAuth(RoleAdmin, s.handleGetDestination))
	s.mux.HandleFunc("PUT /v1/admin/destinations/{name}", s.withAuth(RoleAdmin, s.handleUpdateDestination))
	s.mux.HandleFunc("DELETE /v1/admin/destinations/{name}", s.withAuth(RoleAdmin, s.handleDeleteDestination))

	// Analytics relay endpoints — thin clients POST events; server dispatches to providers.
	s.mux.HandleFunc("POST /v1/track", s.withAuth(RoleViewer, s.handleTrack))
	s.mux.HandleFunc("POST /v1/identify", s.withAuth(RoleViewer, s.handleIdentify))
	s.mux.HandleFunc("POST /v1/group", s.withAuth(RoleViewer, s.handleGroup))
	s.mux.HandleFunc("POST /v1/page", s.withAuth(RoleViewer, s.handlePage))
	s.mux.HandleFunc("POST /v1/alias", s.withAuth(RoleViewer, s.handleAlias))
	s.mux.HandleFunc("POST /v1/batch", s.withAuth(RoleViewer, s.handleBatch))
	s.mux.HandleFunc("POST /v1/flush", s.withAuth(RoleViewer, s.handleFlush))
}

// uiPinger is the DestinationPinger wired into the UI handler.
// It builds a provider for dest, calls Ping if the provider implements
// provider.HealthChecker, and maps ErrUnsupportedOperation to ui.ErrHealthUnknown
// so the status page can distinguish "unreachable" from "unknown".
func (s *Server) uiPinger(ctx context.Context, dest spec.DestinationDef) error {
	err := s.pingDestination(ctx, dest)
	if errors.Is(err, provider.ErrUnsupportedOperation) {
		return ui.ErrHealthUnknown
	}
	if err != nil {
		slog.WarnContext(ctx, "destination health check failed",
			"destination", dest.Name,
			"provider", dest.Provider,
			"error", err,
		)
	}
	return err
}
