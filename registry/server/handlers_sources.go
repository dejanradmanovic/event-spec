package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.st.ListSources(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if sources == nil {
		sources = []spec.SourceDef{}
	}
	jsonOK(w, sources)
}

func (s *Server) handleGetSourceAdmin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	def, err := s.st.GetSource(r.Context(), name)
	if err != nil {
		writeEventError(w, err)
		return
	}
	jsonOK(w, def)
}

func (s *Server) handleCreateSource(w http.ResponseWriter, r *http.Request) {
	var src spec.SourceDef
	if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if src.Name == "" || src.Language == "" {
		jsonError(w, "name and language are required", http.StatusBadRequest)
		return
	}
	userID, _ := r.Context().Value(ctxUserID).(string)
	if err := s.st.CreateSource(r.Context(), src, userID); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(src)
}

func (s *Server) handleUpdateSource(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var src spec.SourceDef
	if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	src.Name = name
	if src.Language == "" {
		jsonError(w, "language is required", http.StatusBadRequest)
		return
	}
	userID, _ := r.Context().Value(ctxUserID).(string)
	if err := s.st.UpdateSource(r.Context(), src, userID); err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			jsonError(w, "source not found", http.StatusNotFound)
			return
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, src)
}

func (s *Server) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	userID, _ := r.Context().Value(ctxUserID).(string)
	if err := s.st.DeleteSource(r.Context(), name, userID); err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			jsonError(w, "source not found", http.StatusNotFound)
			return
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
