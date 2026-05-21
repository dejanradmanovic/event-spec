package ui

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

// Role constants mirror those in the server package.
const (
	RoleViewer    = "viewer"
	RolePublisher = "publisher"
	RoleAdmin     = "admin"
)

const cookieName = "_esui"

type contextKey int

const (
	ctxUserID contextKey = iota
	ctxRole
)

// AuditFilter constrains an audit log query.
type AuditFilter struct {
	Since      *time.Time
	Until      *time.Time
	EntityType string
	UserID     string
	Limit      int
}

// AuditEntry is a single audit log record.
type AuditEntry struct {
	ID         int64
	Action     string
	EntityType string
	EntityID   int64
	UserID     string
	Timestamp  time.Time
	Details    string
}

// APIKeyRecord is the public metadata for a stored API key.
type APIKeyRecord struct {
	ID        int64
	Role      string
	Name      string
	CreatedBy string
	CreatedAt time.Time
	ExpiresAt *time.Time
}

// WebhookRecord is a registered webhook entry.
type WebhookRecord struct {
	ID        int64
	URL       string
	CreatedBy string
	CreatedAt time.Time
}

// ServerSetting is a runtime configuration key-value entry.
type ServerSetting struct {
	Key   string
	Value string
}

// Store is the persistence interface required by the UI handler.
type Store interface {
	ListEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error)
	GetEvent(ctx context.Context, namespace, name, version string) (*spec.EventDef, error)
	// PublishEvent creates or updates an event version and appends an audit entry under userID.
	PublishEvent(ctx context.Context, event spec.EventDef, userID string) error
	LookupAPIKey(ctx context.Context, keyHash string) (userID, role string, err error)
	ListAuditLog(ctx context.Context, filter AuditFilter) ([]AuditEntry, error)
	ListAPIKeys(ctx context.Context) ([]APIKeyRecord, error)
	CreateAPIKey(ctx context.Context, keyHash, role, name, createdBy string, expiresAt *time.Time) (int64, error)
	RevokeAPIKey(ctx context.Context, id int64) error
	ListWebhooksAdmin(ctx context.Context) ([]WebhookRecord, error)
	RegisterWebhook(ctx context.Context, webhookURL, userID string) error
	DeleteWebhook(ctx context.Context, id int64) error
	ListSettings(ctx context.Context) ([]ServerSetting, error)
	SetSetting(ctx context.Context, key, value string) error
}

// HooksEnabled returns the current live value of the hooks_enabled runtime toggle.
type HooksEnabled func() bool

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

// --- auth helpers ---

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
		// Response already started; log internally but cannot change status.
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

// --- login / logout ---

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

// --- dashboard ---

type dashboardData struct {
	baseData
	StatusCounts map[string]int
	RecentAudit  []AuditEntry
}

func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	events, err := h.st.ListEvents(r.Context(), registry.ListFilter{})
	if err != nil {
		h.renderErrorPage(w, r, http.StatusInternalServerError, "Dashboard", err.Error())
		return
	}
	counts := map[string]int{"active": 0, "draft": 0, "deprecated": 0, "deleted": 0}
	for _, e := range events {
		if _, ok := counts[string(e.Status)]; ok {
			counts[string(e.Status)]++
		}
	}
	recent, _ := h.st.ListAuditLog(r.Context(), AuditFilter{Limit: 10})
	b := newBase(r, "Dashboard")
	h.render(w, "dashboard", dashboardData{b, counts, recent})
}

// --- event list ---

type eventsData struct {
	baseData
	Events     []spec.EventDef
	Namespaces []string
	NSFilter   string
	StatFilter string
	Search     string
}

func (h *Handler) handleEventList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ns := q.Get("namespace")
	status := q.Get("status")
	search := strings.ToLower(q.Get("q"))

	events, err := h.st.ListEvents(r.Context(), registry.ListFilter{
		Namespace: ns,
		Status:    spec.EventStatus(status),
	})
	if err != nil {
		h.renderErrorPage(w, r, http.StatusInternalServerError, "Events", err.Error())
		return
	}

	if search != "" {
		filtered := events[:0]
		for _, e := range events {
			if strings.Contains(strings.ToLower(e.Name), search) ||
				strings.Contains(strings.ToLower(e.DisplayName), search) {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	// Collect unique namespaces from all events (unfiltered).
	all, _ := h.st.ListEvents(r.Context(), registry.ListFilter{})
	nsSet := map[string]struct{}{}
	for _, e := range all {
		nsSet[e.Namespace] = struct{}{}
	}
	namespaces := make([]string, 0, len(nsSet))
	for ns := range nsSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	b := newBase(r, "Event Catalog")
	h.render(w, "events", eventsData{b, events, namespaces, ns, status, search})
}

// --- event detail ---

type eventDetailData struct {
	baseData
	Event    *spec.EventDef
	Versions []spec.EventDef
	Props    []propRow
}

type propRow struct {
	Name string
	Def  spec.PropertyDef
}

func (h *Handler) handleEventDetail(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("ns")
	name := r.PathValue("name")

	ev, err := h.st.GetEvent(r.Context(), ns, name, "")
	if err != nil {
		h.renderErrorPage(w, r, http.StatusNotFound, "Event Detail", err.Error())
		return
	}

	all, _ := h.st.ListEvents(r.Context(), registry.ListFilter{Namespace: ns})
	var versions []spec.EventDef
	for _, e := range all {
		if e.Name == name {
			versions = append(versions, e)
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		vi, _ := spec.ParseSchemaVer(versions[i].Version)
		vj, _ := spec.ParseSchemaVer(versions[j].Version)
		return spec.CompareSchemaVer(vi, vj) > 0
	})

	var props []propRow
	propNames := make([]string, 0, len(ev.Properties))
	for pn := range ev.Properties {
		propNames = append(propNames, pn)
	}
	sort.Strings(propNames)
	for _, pn := range propNames {
		props = append(props, propRow{pn, ev.Properties[pn]})
	}

	b := newBase(r, ev.DisplayName)
	if b.Title == "" {
		b.Title = ev.Name
	}
	h.render(w, "event_detail", eventDetailData{b, ev, versions, props})
}

// --- diff view ---

type eventDiffData struct {
	baseData
	Namespace string
	EventName string
	From      string
	To        string
	Changes   []spec.Change
	FromDef   *spec.EventDef
	ToDef     *spec.EventDef
}

func (h *Handler) handleEventDiff(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("ns")
	name := r.PathValue("name")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	b := newBase(r, "Diff: "+name)

	if from == "" || to == "" {
		h.render(w, "event_diff", eventDiffData{baseData: b, Namespace: ns, EventName: name})
		return
	}

	fromDef, err := h.st.GetEvent(r.Context(), ns, name, from)
	if err != nil {
		h.renderErrorPage(w, r, http.StatusNotFound, "Diff", "version "+from+" not found")
		return
	}
	toDef, err := h.st.GetEvent(r.Context(), ns, name, to)
	if err != nil {
		h.renderErrorPage(w, r, http.StatusNotFound, "Diff", "version "+to+" not found")
		return
	}

	changes := spec.Diff(fromDef, toDef)
	h.render(w, "event_diff", eventDiffData{b, ns, name, from, to, changes, fromDef, toDef})
}

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
	expiresIn := r.FormValue("expires_in")

	if roleLevel(role) < 0 || role == "" {
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

	userID, _ := r.Context().Value(ctxUserID).(string)
	id, err := h.st.CreateAPIKey(r.Context(), keyHash, role, name, userID, expiresAt)
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
	url := strings.TrimSpace(r.FormValue("url"))
	if url == "" {
		http.Redirect(w, r, "/ui/settings/webhooks", http.StatusFound)
		return
	}
	userID, _ := r.Context().Value(ctxUserID).(string)
	_ = h.st.RegisterWebhook(r.Context(), url, userID)
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
	// Ensure hooks_enabled always appears even if DB row absent.
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

// --- publisher: event create / edit ---

type eventFormData struct {
	baseData
	YAML      string
	FormError string
	IsEdit    bool
	Namespace string
	EventName string
}

// newEventYAMLTemplate returns a minimal YAML skeleton for new events.
func newEventYAMLTemplate() string {
	return `$schema: "https://event-spec.io/schemas/event/v1"

name: my_event
display_name: "My Event"
description: |
  Describe what this event captures.
version: "1-0-0"
changelog: "Initial version"
status: draft
namespace: my_namespace
tags: []
owner: "team@example.com"
type: track
event_name: "My Event"

properties:
  example_property:
    type: string
    required: true
    description: "An example property"

# property_priority: event_wins
# sampling:
#   strategy: none
#   rate: 1.0
`
}

func (h *Handler) handleNewEventForm(w http.ResponseWriter, r *http.Request) {
	b := newBase(r, "Publish New Event")
	h.render(w, "event_form", eventFormData{baseData: b, YAML: newEventYAMLTemplate()})
}

func (h *Handler) handlePublishNewEvent(w http.ResponseWriter, r *http.Request) {
	h.submitEventForm(w, r, "", "", false)
}

func (h *Handler) handleEditEventForm(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("ns")
	name := r.PathValue("name")

	ev, err := h.st.GetEvent(r.Context(), ns, name, "")
	if err != nil {
		h.renderErrorPage(w, r, http.StatusNotFound, "Edit Event", err.Error())
		return
	}

	raw, err := yaml.Marshal(ev)
	if err != nil {
		h.renderErrorPage(w, r, http.StatusInternalServerError, "Edit Event", err.Error())
		return
	}

	b := newBase(r, "Edit "+ev.Name)
	h.render(w, "event_form", eventFormData{
		baseData:  b,
		YAML:      string(raw),
		IsEdit:    true,
		Namespace: ns,
		EventName: name,
	})
}

func (h *Handler) handlePublishEventEdit(w http.ResponseWriter, r *http.Request) {
	h.submitEventForm(w, r, r.PathValue("ns"), r.PathValue("name"), true)
}

// submitEventForm parses the YAML form body, validates, publishes, and redirects on success.
func (h *Handler) submitEventForm(w http.ResponseWriter, r *http.Request, nsHint, nameHint string, isEdit bool) {
	if err := r.ParseForm(); err != nil {
		h.renderErrorPage(w, r, http.StatusBadRequest, "Publish Event", "Invalid form submission.")
		return
	}

	rawYAML := r.FormValue("spec_yaml")
	var ev spec.EventDef
	if err := yaml.Unmarshal([]byte(rawYAML), &ev); err != nil {
		b := newBase(r, "Publish Event")
		h.render(w, "event_form", eventFormData{
			baseData:  b,
			YAML:      rawYAML,
			FormError: "Invalid YAML: " + err.Error(),
			IsEdit:    isEdit,
			Namespace: nsHint,
			EventName: nameHint,
		})
		return
	}

	if ev.Name == "" || ev.Namespace == "" || ev.Version == "" {
		b := newBase(r, "Publish Event")
		h.render(w, "event_form", eventFormData{
			baseData:  b,
			YAML:      rawYAML,
			FormError: "name, namespace, and version are required.",
			IsEdit:    isEdit,
			Namespace: nsHint,
			EventName: nameHint,
		})
		return
	}

	if ev.Status == "" {
		ev.Status = spec.StatusDraft
	}

	userID, _ := r.Context().Value(ctxUserID).(string)
	if err := h.st.PublishEvent(r.Context(), ev, userID); err != nil {
		b := newBase(r, "Publish Event")
		h.render(w, "event_form", eventFormData{
			baseData:  b,
			YAML:      rawYAML,
			FormError: "Publish failed: " + err.Error(),
			IsEdit:    isEdit,
			Namespace: nsHint,
			EventName: nameHint,
		})
		return
	}

	http.Redirect(w, r, "/ui/events/"+ev.Namespace+"/"+ev.Name, http.StatusFound)
}

// parseSimpleDuration parses "Nd" (days) and "Ny" (years) in addition to Go duration strings.
func parseSimpleDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || n <= 0 {
			return 0, &strconv.NumError{Func: "parseSimpleDuration", Num: s}
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	if strings.HasSuffix(s, "y") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "y"))
		if err != nil || n <= 0 {
			return 0, &strconv.NumError{Func: "parseSimpleDuration", Num: s}
		}
		return time.Duration(n) * 365 * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
