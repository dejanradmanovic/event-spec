package ui

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dejanradmanovic/event-spec/spec"
)

type eventFormData struct {
	baseData
	YAML                  string
	FormError             string
	IsEdit                bool
	Namespace             string
	EventName             string
	AvailableDestinations []string
}

func (h *Handler) handleNewEventForm(w http.ResponseWriter, r *http.Request) {
	b := newBase(r, "Publish New Event")
	dests, _ := h.st.ListDestinations(r.Context())
	h.render(w, "event_form", eventFormData{baseData: b, YAML: newEventYAMLTemplate(), AvailableDestinations: dests})
}

func (h *Handler) handlePublishNewEvent(w http.ResponseWriter, r *http.Request) {
	h.submitEventForm(w, r, "", "", false)
}

func (h *Handler) handleEditEventForm(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("ns")
	name := r.PathValue("name")

	ev, err := h.getEventLatest(r.Context(), ns, name)
	if err != nil {
		h.renderErrorPage(w, r, http.StatusNotFound, "Edit Event", err.Error())
		return
	}

	raw, err := yaml.Marshal(ev)
	if err != nil {
		h.renderErrorPage(w, r, http.StatusInternalServerError, "Edit Event", err.Error())
		return
	}

	dests, _ := h.st.ListDestinations(r.Context())
	b := newBase(r, "Edit "+ev.Name)
	h.render(w, "event_form", eventFormData{
		baseData:              b,
		YAML:                  string(raw),
		IsEdit:                true,
		Namespace:             ns,
		EventName:             name,
		AvailableDestinations: dests,
	})
}

func (h *Handler) handlePublishEventEdit(w http.ResponseWriter, r *http.Request) {
	h.submitEventForm(w, r, r.PathValue("ns"), r.PathValue("name"), true)
}

// submitEventForm parses the YAML form body, validates with spec.ValidateEventDef and
// spec.ValidateVersionBump (for edits), then publishes on success.
func (h *Handler) submitEventForm(w http.ResponseWriter, r *http.Request, nsHint, nameHint string, isEdit bool) {
	if err := r.ParseForm(); err != nil {
		h.renderErrorPage(w, r, http.StatusBadRequest, "Publish Event", "Invalid form submission.")
		return
	}

	rawYAML := r.FormValue("spec_yaml")

	renderFormErr := func(msg string) {
		b := newBase(r, "Publish Event")
		h.render(w, "event_form", eventFormData{
			baseData:  b,
			YAML:      rawYAML,
			FormError: msg,
			IsEdit:    isEdit,
			Namespace: nsHint,
			EventName: nameHint,
		})
	}

	var ev spec.EventDef
	dec := yaml.NewDecoder(strings.NewReader(rawYAML))
	dec.KnownFields(true)
	if err := dec.Decode(&ev); err != nil {
		renderFormErr("Invalid YAML: " + err.Error())
		return
	}

	// Structural validation via spec package.
	if verrs := spec.ValidateEventDef(&ev); len(verrs) > 0 {
		msgs := make([]string, len(verrs))
		for i, e := range verrs {
			msgs[i] = e.Field + ": " + e.Message
		}
		renderFormErr("Validation errors:\n• " + strings.Join(msgs, "\n• "))
		return
	}

	// Lock name and namespace to the URL path in edit mode — they are the event's
	// identity and cannot be changed via an edit.
	if isEdit && nsHint != "" && nameHint != "" {
		ev.Namespace = nsHint
		ev.Name = nameHint
	}

	// For edits, enforce that the version bump is consistent with the detected changes.
	if isEdit && nsHint != "" && nameHint != "" {
		prev, err := h.getEventLatest(r.Context(), nsHint, nameHint)
		if err == nil {
			changes := spec.Diff(prev, &ev)
			if bumpErr := spec.ValidateVersionBump(prev, &ev, changes); bumpErr != nil {
				prevVer, _ := spec.ParseSchemaVer(prev.Version)
				suggested := spec.SuggestVersion(prevVer, changes)
				renderFormErr(fmt.Sprintf("%s (suggested: %s)", bumpErr.Error(), suggested.Raw))
				return
			}
		}
	}

	if ev.Status == "" {
		ev.Status = spec.StatusDraft
	}

	userID, _ := r.Context().Value(ctxUserID).(string)
	if err := h.st.PublishEvent(r.Context(), ev, userID); err != nil {
		renderFormErr("Publish failed: " + err.Error())
		return
	}

	http.Redirect(w, r, "/ui/events/"+ev.Namespace+"/"+ev.Name, http.StatusFound)
}

// newEventYAMLTemplate returns a minimal YAML skeleton for new events.
func newEventYAMLTemplate() string {
	return `$schema: "https://event-spec.io/schemas/event/v1"

namespace: my_namespace
name: my_event
event_name: "My Event"
display_name: "My Event"
description: |
  Describe what this event captures.
version: "1-0-0"
changelog: "Initial version"
status: draft
tags: []
owner: "team@example.com"
type: track

properties:
  example_property:
    type: string
    required: true
    description: "An example property"

# property_priority: event_wins   # event_wins | context_wins | merge
# sampling:
#   strategy: none                 # none | user_id_hash | random
#   rate: 1.0
`
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
