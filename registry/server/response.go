package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dejanradmanovic/event-spec/registry"
)

func writeEventError(w http.ResponseWriter, err error) {
	if errors.Is(err, registry.ErrNotFound) {
		jsonError(w, err.Error(), http.StatusNotFound)
	} else {
		jsonError(w, err.Error(), http.StatusInternalServerError)
	}
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck // response write errors are not actionable from a handler
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck // response write errors are not actionable from a handler
}
