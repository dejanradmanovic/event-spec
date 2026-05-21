package ui

import (
	"html/template"
	"io/fs"
	"net/http"
)

// Handler is the HTTP handler for the web admin UI.
type Handler struct {
	st         Store
	hooksState HooksEnabled
	mux        *http.ServeMux
}

// New creates a Handler backed by st.
func New(st Store, hooksEnabled HooksEnabled) *Handler {
	h := &Handler{
		st:         st,
		hooksState: hooksEnabled,
		mux:        http.NewServeMux(),
	}
	h.routes()
	return h
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) routes() {
	staticSub, _ := fs.Sub(FS, "static")
	h.mux.Handle("GET /ui/static/", http.StripPrefix("/ui/static/", http.FileServer(http.FS(staticSub))))

	h.mux.HandleFunc("GET /ui/login", h.handleLoginForm)
	h.mux.HandleFunc("POST /ui/login", h.handleLogin)
	h.mux.HandleFunc("POST /ui/logout", h.handleLogout)
	h.mux.HandleFunc("GET /ui/bootstrap", h.handleBootstrapForm)
	h.mux.HandleFunc("POST /ui/bootstrap", h.handleBootstrap)

	h.mux.HandleFunc("GET /ui/", h.withSession(RoleViewer, h.handleDashboard))
	h.mux.HandleFunc("GET /ui/events", h.withSession(RoleViewer, h.handleEventList))
	h.mux.HandleFunc("GET /ui/events/new", h.withSession(RolePublisher, h.handleNewEventForm))
	h.mux.HandleFunc("POST /ui/events/new", h.withSession(RolePublisher, h.handlePublishNewEvent))
	h.mux.HandleFunc("GET /ui/events/{ns}/{name}", h.withSession(RoleViewer, h.handleEventDetail))
	h.mux.HandleFunc("GET /ui/events/{ns}/{name}/diff", h.withSession(RoleViewer, h.handleEventDiff))
	h.mux.HandleFunc("GET /ui/events/{ns}/{name}/edit", h.withSession(RolePublisher, h.handleEditEventForm))
	h.mux.HandleFunc("POST /ui/events/{ns}/{name}/edit", h.withSession(RolePublisher, h.handlePublishEventEdit))
	h.mux.HandleFunc("GET /ui/audit", h.withSession(RoleAdmin, h.handleAudit))
	h.mux.HandleFunc("GET /ui/settings/keys", h.withSession(RoleAdmin, h.handleKeys))
	h.mux.HandleFunc("POST /ui/settings/keys", h.withSession(RoleAdmin, h.handleCreateKey))
	h.mux.HandleFunc("DELETE /ui/settings/keys/{id}", h.withSession(RoleAdmin, h.handleRevokeKey))
	h.mux.HandleFunc("GET /ui/settings/webhooks", h.withSession(RoleAdmin, h.handleWebhooks))
	h.mux.HandleFunc("POST /ui/settings/webhooks", h.withSession(RoleAdmin, h.handleAddWebhook))
	h.mux.HandleFunc("DELETE /ui/settings/webhooks/{id}", h.withSession(RoleAdmin, h.handleRemoveWebhook))
	h.mux.HandleFunc("GET /ui/settings/config", h.withSession(RoleAdmin, h.handleConfig))
	h.mux.HandleFunc("PUT /ui/settings/config/{key}", h.withSession(RoleAdmin, h.handleSetConfig))
}

// --- template rendering ---

type baseData struct {
	Title  string
	UserID string
	Role   string
	Flash  string
	IsErr  bool
}

func newBase(r *http.Request, title string) baseData {
	userID, _ := r.Context().Value(ctxUserID).(string)
	role, _ := r.Context().Value(ctxRole).(string)
	return baseData{Title: title, UserID: userID, Role: role}
}

func (h *Handler) render(w http.ResponseWriter, page string, data any) {
	t, err := template.New("").ParseFS(FS, "templates/base.html", "templates/"+page+".html")
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		_ = err
	}
}

func (h *Handler) renderPartial(w http.ResponseWriter, page, block string, data any) {
	t, err := template.New("").ParseFS(FS, "templates/"+page+".html")
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = t.ExecuteTemplate(w, block, data)
}

func (h *Handler) renderLogin(w http.ResponseWriter, flash string, isErr bool) {
	t, err := template.New("").ParseFS(FS, "templates/login.html")
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = t.ExecuteTemplate(w, "login", struct {
		Flash string
		IsErr bool
	}{flash, isErr})
}

func (h *Handler) renderErrorPage(w http.ResponseWriter, r *http.Request, status int, title, msg string) {
	b := newBase(r, title)
	w.WriteHeader(status)
	h.render(w, "error", struct {
		baseData
		Message string
	}{b, msg})
}
