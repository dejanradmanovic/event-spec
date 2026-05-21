package server

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

	// Analytics relay endpoints — thin clients POST events; server dispatches to providers.
	s.mux.HandleFunc("POST /v1/track", s.withAuth(RoleViewer, s.handleTrack))
	s.mux.HandleFunc("POST /v1/identify", s.withAuth(RoleViewer, s.handleIdentify))
	s.mux.HandleFunc("POST /v1/group", s.withAuth(RoleViewer, s.handleGroup))
	s.mux.HandleFunc("POST /v1/page", s.withAuth(RoleViewer, s.handlePage))
	s.mux.HandleFunc("POST /v1/alias", s.withAuth(RoleViewer, s.handleAlias))
	s.mux.HandleFunc("POST /v1/batch", s.withAuth(RoleViewer, s.handleBatch))
	s.mux.HandleFunc("POST /v1/flush", s.withAuth(RoleViewer, s.handleFlush))
}
