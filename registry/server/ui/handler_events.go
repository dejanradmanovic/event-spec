package ui

import (
	"net/http"
	"sort"
	"strings"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

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

	// Collect unique namespaces from all events (unfiltered) for the filter dropdown.
	all, _ := h.st.ListEvents(r.Context(), registry.ListFilter{})
	nsSet := map[string]struct{}{}
	for _, e := range all {
		nsSet[e.Namespace] = struct{}{}
	}
	namespaces := make([]string, 0, len(nsSet))
	for n := range nsSet {
		namespaces = append(namespaces, n)
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

	propNames := make([]string, 0, len(ev.Properties))
	for pn := range ev.Properties {
		propNames = append(propNames, pn)
	}
	sort.Strings(propNames)
	props := make([]propRow, 0, len(propNames))
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
