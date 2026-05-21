package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

func (s *Server) handleListDestinations(w http.ResponseWriter, r *http.Request) {
	dests, err := s.st.ListDestinationsFull(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if dests == nil {
		dests = []spec.DestinationDef{}
	}
	jsonOK(w, dests)
}

func (s *Server) handleGetDestination(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	def, err := s.st.GetDestination(r.Context(), name)
	if err != nil {
		writeEventError(w, err)
		return
	}
	jsonOK(w, def)
}

func (s *Server) handleCreateDestination(w http.ResponseWriter, r *http.Request) {
	var dest spec.DestinationDef
	if err := json.NewDecoder(r.Body).Decode(&dest); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if dest.Name == "" || dest.Provider == "" {
		jsonError(w, "name and provider are required", http.StatusBadRequest)
		return
	}
	userID, _ := r.Context().Value(ctxUserID).(string)
	if err := s.st.CreateDestination(r.Context(), dest, userID); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(dest)
}

func (s *Server) handleUpdateDestination(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var dest spec.DestinationDef
	if err := json.NewDecoder(r.Body).Decode(&dest); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	dest.Name = name
	if dest.Provider == "" {
		jsonError(w, "provider is required", http.StatusBadRequest)
		return
	}
	userID, _ := r.Context().Value(ctxUserID).(string)
	if err := s.st.UpdateDestination(r.Context(), dest, userID); err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			jsonError(w, "destination not found", http.StatusNotFound)
			return
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, dest)
}

func (s *Server) handleDeleteDestination(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	userID, _ := r.Context().Value(ctxUserID).(string)
	if err := s.st.DeleteDestination(r.Context(), name, userID); err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			jsonError(w, "destination not found", http.StatusNotFound)
			return
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
