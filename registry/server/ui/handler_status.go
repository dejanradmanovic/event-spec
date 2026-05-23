package ui

import (
	"errors"
	"html/template"
	"net/http"
)

type destinationStatus struct {
	Name         string
	ProviderType string
	Status       string // "reachable", "unreachable", "unknown"
	ErrMsg       string // non-empty when Status == "unreachable"
}

type statusPageData struct {
	Uptime       string
	Destinations []destinationStatus
}

func (h *Handler) handleStatusPage(w http.ResponseWriter, r *http.Request) {
	dests, err := h.st.ListDestinationsFull(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	statuses := make([]destinationStatus, 0, len(dests))
	for _, d := range dests {
		ds := destinationStatus{Name: d.Name, ProviderType: d.Provider, Status: "unknown"}
		if h.pinger != nil {
			if pingErr := h.pinger(r.Context(), d); pingErr != nil {
				if errors.Is(pingErr, ErrHealthUnknown) {
					ds.Status = "unknown"
				} else {
					ds.Status = "unreachable"
					ds.ErrMsg = pingErr.Error()
				}
			} else {
				ds.Status = "reachable"
			}
		}
		statuses = append(statuses, ds)
	}

	var uptime string
	if h.uptime != nil {
		uptime = h.uptime().String()
	}

	t, err := template.New("").ParseFS(FS, "templates/status.html")
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = t.ExecuteTemplate(w, "status", statusPageData{Uptime: uptime, Destinations: statuses})
}
