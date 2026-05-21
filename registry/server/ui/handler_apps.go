package ui

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"

	"gopkg.in/yaml.v3"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

type appsData struct {
	baseData
	Apps []spec.SourceDef
}

type appFormData struct {
	baseData
	YAML              string
	FormError         string
	IsEdit            bool
	AppName           string
	Destinations      []spec.DestinationDef
	EventVersionsJSON template.JS
}

type appDetailData struct {
	baseData
	App *spec.SourceDef
}

// buildEventVersionsJSON returns a JSON object mapping "namespace/name" → [version, ...]
// for use in the Visual Editor's version-pinning dropdown.
func buildEventVersionsJSON(events []spec.EventDef) template.JS {
	evMap := make(map[string][]string)
	for _, ev := range events {
		path := ev.Namespace + "/" + ev.Name
		found := false
		for _, v := range evMap[path] {
			if v == ev.Version {
				found = true
				break
			}
		}
		if !found {
			evMap[path] = append(evMap[path], ev.Version)
		}
	}
	b, _ := json.Marshal(evMap)
	return template.JS(b)
}

func (h *Handler) loadFormExtras(r *http.Request) ([]spec.DestinationDef, template.JS) {
	dests, _ := h.st.ListDestinationsFull(r.Context())
	events, _ := h.st.ListAllEvents(r.Context(), registry.ListFilter{})
	return dests, buildEventVersionsJSON(events)
}

func (h *Handler) handleAppList(w http.ResponseWriter, r *http.Request) {
	apps, err := h.st.ListSources(r.Context())
	if err != nil {
		h.renderErrorPage(w, r, http.StatusInternalServerError, "Apps", err.Error())
		return
	}
	b := newBase(r, "Apps")
	h.render(w, "apps", appsData{b, apps})
}

func (h *Handler) handleNewAppForm(w http.ResponseWriter, r *http.Request) {
	dests, evJSON := h.loadFormExtras(r)
	b := newBase(r, "New App")
	h.render(w, "app_form", appFormData{
		baseData:          b,
		YAML:              newAppYAMLTemplate(),
		Destinations:      dests,
		EventVersionsJSON: evJSON,
	})
}

func (h *Handler) handleCreateApp(w http.ResponseWriter, r *http.Request) {
	h.submitAppForm(w, r, "", false)
}

func (h *Handler) handleAppDetail(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	app, err := h.st.GetSource(r.Context(), name)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			h.renderErrorPage(w, r, http.StatusNotFound, "App", "App not found.")
			return
		}
		h.renderErrorPage(w, r, http.StatusInternalServerError, "App", err.Error())
		return
	}
	b := newBase(r, app.Name)
	h.render(w, "app_details", appDetailData{b, app})
}

func (h *Handler) handleEditAppForm(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	app, err := h.st.GetSource(r.Context(), name)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			h.renderErrorPage(w, r, http.StatusNotFound, "Edit App", "App not found.")
			return
		}
		h.renderErrorPage(w, r, http.StatusInternalServerError, "Edit App", err.Error())
		return
	}
	raw, err := yaml.Marshal(app)
	if err != nil {
		h.renderErrorPage(w, r, http.StatusInternalServerError, "Edit App", err.Error())
		return
	}
	dests, evJSON := h.loadFormExtras(r)
	b := newBase(r, "Edit "+name)
	h.render(w, "app_form", appFormData{
		baseData:          b,
		YAML:              string(raw),
		IsEdit:            true,
		AppName:           name,
		Destinations:      dests,
		EventVersionsJSON: evJSON,
	})
}

func (h *Handler) handleUpdateApp(w http.ResponseWriter, r *http.Request) {
	h.submitAppForm(w, r, r.PathValue("name"), true)
}

func (h *Handler) handleDeleteApp(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	userID, _ := r.Context().Value(ctxUserID).(string)
	_ = h.st.DeleteSource(r.Context(), name, userID)
	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/ui/settings/apps", http.StatusFound)
}

func (h *Handler) submitAppForm(w http.ResponseWriter, r *http.Request, nameHint string, isEdit bool) {
	if err := r.ParseForm(); err != nil {
		h.renderErrorPage(w, r, http.StatusBadRequest, "App", "Invalid form submission.")
		return
	}
	rawYAML := r.FormValue("spec_yaml")

	renderFormErr := func(msg string) {
		title := "New App"
		if isEdit {
			title = "Edit " + nameHint
		}
		b := newBase(r, title)
		dests, evJSON := h.loadFormExtras(r)
		h.render(w, "app_form", appFormData{
			baseData:          b,
			YAML:              rawYAML,
			FormError:         msg,
			IsEdit:            isEdit,
			AppName:           nameHint,
			Destinations:      dests,
			EventVersionsJSON: evJSON,
		})
	}

	var app spec.SourceDef
	if err := yaml.Unmarshal([]byte(rawYAML), &app); err != nil {
		renderFormErr("Invalid YAML: " + err.Error())
		return
	}
	if app.Name == "" {
		renderFormErr("name is required")
		return
	}
	if app.Language == "" {
		renderFormErr("language is required")
		return
	}
	if isEdit {
		app.Name = nameHint
	}

	userID, _ := r.Context().Value(ctxUserID).(string)
	var opErr error
	if isEdit {
		opErr = h.st.UpdateSource(r.Context(), app, userID)
	} else {
		opErr = h.st.CreateSource(r.Context(), app, userID)
	}
	if opErr != nil {
		renderFormErr("Operation failed: " + opErr.Error())
		return
	}
	http.Redirect(w, r, "/ui/settings/apps", http.StatusFound)
}

func newAppYAMLTemplate() string {
	return `name: my-app
platform: web
language: typescript
events:
  - ecommerce/**
destinations:
  - amplitude
output:
  path: ./src/analytics/generated
  package: "@my-company/analytics"
`
}
