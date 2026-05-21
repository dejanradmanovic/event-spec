package ui

import (
	"errors"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

type destinationsData struct {
	baseData
	Destinations []spec.DestinationDef
}

type destinationFormData struct {
	baseData
	YAML      string
	FormError string
	IsEdit    bool
	DestName  string
}

type destinationDetailData struct {
	baseData
	Dest      *spec.DestinationDef
	ConfigKVs []configKV
}

type configKV struct {
	Key   string
	Value string
}

func (h *Handler) handleDestinationList(w http.ResponseWriter, r *http.Request) {
	dests, err := h.st.ListDestinationsFull(r.Context())
	if err != nil {
		h.renderErrorPage(w, r, http.StatusInternalServerError, "Destinations", err.Error())
		return
	}
	b := newBase(r, "Destinations")
	h.render(w, "destinations", destinationsData{b, dests})
}

func (h *Handler) handleNewDestinationForm(w http.ResponseWriter, r *http.Request) {
	b := newBase(r, "New Destination")
	h.render(w, "destination_form", destinationFormData{baseData: b, YAML: newDestinationYAMLTemplate()})
}

func (h *Handler) handleCreateDestination(w http.ResponseWriter, r *http.Request) {
	h.submitDestinationForm(w, r, "", false)
}

func (h *Handler) handleDestinationDetail(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	dest, err := h.st.GetDestination(r.Context(), name)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			h.renderErrorPage(w, r, http.StatusNotFound, "Destination", "Destination not found.")
			return
		}
		h.renderErrorPage(w, r, http.StatusInternalServerError, "Destination", err.Error())
		return
	}
	kvs := make([]configKV, 0, len(dest.Config))
	for k, v := range dest.Config {
		kvs = append(kvs, configKV{Key: k, Value: toString(v)})
	}
	b := newBase(r, dest.Name)
	h.render(w, "destination_details", destinationDetailData{b, dest, kvs})
}

func (h *Handler) handleEditDestinationForm(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	dest, err := h.st.GetDestination(r.Context(), name)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			h.renderErrorPage(w, r, http.StatusNotFound, "Edit Destination", "Destination not found.")
			return
		}
		h.renderErrorPage(w, r, http.StatusInternalServerError, "Edit Destination", err.Error())
		return
	}
	raw, err := yaml.Marshal(dest)
	if err != nil {
		h.renderErrorPage(w, r, http.StatusInternalServerError, "Edit Destination", err.Error())
		return
	}
	b := newBase(r, "Edit "+name)
	h.render(w, "destination_form", destinationFormData{
		baseData: b,
		YAML:     string(raw),
		IsEdit:   true,
		DestName: name,
	})
}

func (h *Handler) handleUpdateDestination(w http.ResponseWriter, r *http.Request) {
	h.submitDestinationForm(w, r, r.PathValue("name"), true)
}

func (h *Handler) handleDeleteDestination(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	userID, _ := r.Context().Value(ctxUserID).(string)
	_ = h.st.DeleteDestination(r.Context(), name, userID)
	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/ui/settings/destinations", http.StatusFound)
}

func (h *Handler) submitDestinationForm(w http.ResponseWriter, r *http.Request, nameHint string, isEdit bool) {
	if err := r.ParseForm(); err != nil {
		h.renderErrorPage(w, r, http.StatusBadRequest, "Destination", "Invalid form submission.")
		return
	}
	rawYAML := r.FormValue("spec_yaml")

	renderFormErr := func(msg string) {
		title := "New Destination"
		if isEdit {
			title = "Edit " + nameHint
		}
		b := newBase(r, title)
		h.render(w, "destination_form", destinationFormData{
			baseData:  b,
			YAML:      rawYAML,
			FormError: msg,
			IsEdit:    isEdit,
			DestName:  nameHint,
		})
	}

	var dest spec.DestinationDef
	if err := yaml.Unmarshal([]byte(rawYAML), &dest); err != nil {
		renderFormErr("Invalid YAML: " + err.Error())
		return
	}
	if dest.Name == "" {
		renderFormErr("name is required")
		return
	}
	if dest.Provider == "" {
		renderFormErr("provider is required")
		return
	}
	if isEdit {
		dest.Name = nameHint
	}

	userID, _ := r.Context().Value(ctxUserID).(string)
	var opErr error
	if isEdit {
		opErr = h.st.UpdateDestination(r.Context(), dest, userID)
	} else {
		opErr = h.st.CreateDestination(r.Context(), dest, userID)
	}
	if opErr != nil {
		renderFormErr("Operation failed: " + opErr.Error())
		return
	}
	http.Redirect(w, r, "/ui/settings/destinations", http.StatusFound)
}

func newDestinationYAMLTemplate() string {
	return `name: my_destination
provider: amplitude
config:
  api_key: "${MY_API_KEY}"
  secret_type: env_var
`
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		b, _ := yaml.Marshal(v)
		return strings.TrimSpace(string(b))
	}
}
