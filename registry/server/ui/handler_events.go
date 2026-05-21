package ui

import (
	"context"
	"fmt"
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
	if r.URL.Path != "/ui/" {
		h.renderErrorPage(w, r, http.StatusNotFound, "Page Not Found", "The page you're looking for doesn't exist.")
		return
	}
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
	var recent []AuditEntry
	if role, _ := r.Context().Value(ctxRole).(string); role == RoleAdmin {
		recent, _ = h.st.ListAuditLog(r.Context(), AuditFilter{Limit: 10})
	}
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

	sort.Slice(events, func(i, j int) bool {
		if events[i].Namespace != events[j].Namespace {
			return events[i].Namespace < events[j].Namespace
		}
		return events[i].Name < events[j].Name
	})

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

// getEventLatest returns the highest-versioned event for ns/name regardless of status.
// ListEvents deduplicates to one per (namespace, name), so a simple name lookup suffices.
func (h *Handler) getEventLatest(ctx context.Context, ns, name string) (*spec.EventDef, error) {
	events, err := h.st.ListEvents(ctx, registry.ListFilter{Namespace: ns})
	if err != nil {
		return nil, err
	}
	for i := range events {
		if events[i].Name == name {
			return &events[i], nil
		}
	}
	return nil, fmt.Errorf("event %s/%s: not found", ns, name)
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

	ev, err := h.getEventLatest(r.Context(), ns, name)
	if err != nil {
		h.renderErrorPage(w, r, http.StatusNotFound, "Event Detail", err.Error())
		return
	}

	allVersions, _ := h.st.ListAllEvents(r.Context(), registry.ListFilter{Namespace: ns})
	var versions []spec.EventDef
	for _, e := range allVersions {
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
	Versions  []spec.EventDef
	Changes   []spec.Change
	FromDef   *spec.EventDef
	ToDef     *spec.EventDef
}

func (h *Handler) handleEventDiff(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("ns")
	name := r.PathValue("name")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	allVersions, _ := h.st.ListAllEvents(r.Context(), registry.ListFilter{Namespace: ns})
	var versions []spec.EventDef
	for _, e := range allVersions {
		if e.Name == name {
			versions = append(versions, e)
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		vi, _ := spec.ParseSchemaVer(versions[i].Version)
		vj, _ := spec.ParseSchemaVer(versions[j].Version)
		return spec.CompareSchemaVer(vi, vj) > 0
	})

	// Auto-populate: default to comparing second-latest against latest.
	if to == "" && len(versions) >= 1 {
		to = versions[0].Version
	}
	if from == "" && len(versions) >= 2 {
		from = versions[1].Version
	}

	b := newBase(r, "Diff: "+name)
	data := eventDiffData{
		baseData:  b,
		Namespace: ns,
		EventName: name,
		From:      from,
		To:        to,
		Versions:  versions,
	}

	if from != "" && to != "" {
		fromDef, err := h.st.GetEvent(r.Context(), ns, name, from)
		if err == nil {
			toDef, err := h.st.GetEvent(r.Context(), ns, name, to)
			if err == nil {
				data.FromDef = fromDef
				data.ToDef = toDef
				data.Changes = spec.Diff(fromDef, toDef)
			}
		}
	}

	h.render(w, "event_diff", data)
}
