package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/dejanradmanovic/event-spec/analytics"
	"github.com/dejanradmanovic/event-spec/provider"
	"github.com/dejanradmanovic/event-spec/provider/amplitude"
	"github.com/dejanradmanovic/event-spec/provider/noop"
	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

// clientForSource returns (or lazily builds) the analytics.Client for sourceName.
// The client is constructed from the source's destinations stored in the DB and cached
// for the lifetime of the server. Thread-safe via double-checked locking.
func (s *Server) clientForSource(ctx context.Context, sourceName string) (*analytics.Client, error) {
	s.clientsMu.RLock()
	c, ok := s.clients[sourceName]
	s.clientsMu.RUnlock()
	if ok {
		return c, nil
	}

	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	if c, ok = s.clients[sourceName]; ok {
		return c, nil
	}

	src, err := s.st.GetSource(ctx, sourceName)
	if err != nil {
		return nil, err
	}

	var ps []provider.Provider
	for _, destName := range src.Destinations {
		dest, err := s.st.GetDestination(ctx, destName)
		if err != nil {
			continue
		}
		p, err := buildProvider(dest)
		if err != nil {
			continue
		}
		ps = append(ps, p)
	}

	if len(ps) == 0 {
		ps = []provider.Provider{noop.New()}
	}

	c = analytics.NewClient(analytics.WithProviders(ps...))
	s.clients[sourceName] = c
	return c, nil
}

// buildProvider constructs a provider.Provider from a DestinationDef.
// Unknown provider names fall back to the noop provider so the server
// degrades gracefully when new provider types are added to the spec.
func buildProvider(dest *spec.DestinationDef) (provider.Provider, error) {
	switch dest.Provider {
	case "amplitude":
		cfg := provider.ProviderConfig{SecretType: provider.SecretInline}
		if dest.Config != nil {
			if v, ok := dest.Config["api_key"].(string); ok {
				cfg.APIKey = v
			}
		}
		return amplitude.New(amplitude.Config{ProviderConfig: cfg})
	case "noop":
		return noop.New(), nil
	default:
		return noop.New(), nil
	}
}

// mergeEventContext combines a batch-level context with a per-item context.
// Per-item non-empty fields win; Attributes are merged key-by-key with per-item keys winning.
func mergeEventContext(base, override EventContext) EventContext {
	result := base
	if override.UserID != "" {
		result.UserID = override.UserID
	}
	if override.AnonymousID != "" {
		result.AnonymousID = override.AnonymousID
	}
	if len(override.Attributes) > 0 {
		merged := make(map[string]any, len(base.Attributes)+len(override.Attributes))
		for k, v := range base.Attributes {
			merged[k] = v
		}
		for k, v := range override.Attributes {
			merged[k] = v
		}
		result.Attributes = merged
	}
	return result
}

func sourceErrResponse(w http.ResponseWriter, sourceName string, err error) {
	if errors.Is(err, registry.ErrNotFound) {
		jsonError(w, "source not found: "+sourceName, http.StatusBadRequest)
	} else {
		jsonError(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleTrack(w http.ResponseWriter, r *http.Request) {
	var req TrackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Source == "" || req.EventName == "" {
		jsonError(w, "source and event_name are required", http.StatusBadRequest)
		return
	}

	c, err := s.clientForSource(r.Context(), req.Source)
	if err != nil {
		sourceErrResponse(w, req.Source, err)
		return
	}

	if err := c.Track(r.Context(), analytics.Event{
		Name:       req.EventName,
		Properties: req.Properties,
	}, analytics.WithContextOverride(analytics.AnalyticsContext{
		UserID:      req.Context.UserID,
		AnonymousID: req.Context.AnonymousID,
		Attributes:  req.Context.Attributes,
	})); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleIdentify(w http.ResponseWriter, r *http.Request) {
	var req IdentifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Source == "" {
		jsonError(w, "source is required", http.StatusBadRequest)
		return
	}

	c, err := s.clientForSource(r.Context(), req.Source)
	if err != nil {
		sourceErrResponse(w, req.Source, err)
		return
	}

	if err := c.Identify(r.Context(), req.UserID, req.Traits,
		analytics.WithContextOverride(analytics.AnalyticsContext{
			UserID:      req.Context.UserID,
			AnonymousID: req.Context.AnonymousID,
			Attributes:  req.Context.Attributes,
		})); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleGroup(w http.ResponseWriter, r *http.Request) {
	var req GroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Source == "" {
		jsonError(w, "source is required", http.StatusBadRequest)
		return
	}

	c, err := s.clientForSource(r.Context(), req.Source)
	if err != nil {
		sourceErrResponse(w, req.Source, err)
		return
	}

	if err := c.Group(r.Context(), req.GroupID, req.Traits,
		analytics.WithContextOverride(analytics.AnalyticsContext{
			UserID:      req.Context.UserID,
			AnonymousID: req.Context.AnonymousID,
			Attributes:  req.Context.Attributes,
		})); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	var req PageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Source == "" || req.Name == "" {
		jsonError(w, "source and name are required", http.StatusBadRequest)
		return
	}

	c, err := s.clientForSource(r.Context(), req.Source)
	if err != nil {
		sourceErrResponse(w, req.Source, err)
		return
	}

	if err := c.Page(r.Context(), req.Name, req.Properties,
		analytics.WithContextOverride(analytics.AnalyticsContext{
			UserID:      req.Context.UserID,
			AnonymousID: req.Context.AnonymousID,
			Attributes:  req.Context.Attributes,
		})); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleAlias(w http.ResponseWriter, r *http.Request) {
	var req AliasRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Source == "" {
		jsonError(w, "source is required", http.StatusBadRequest)
		return
	}

	c, err := s.clientForSource(r.Context(), req.Source)
	if err != nil {
		sourceErrResponse(w, req.Source, err)
		return
	}

	if err := c.Alias(r.Context(), req.UserID, req.PreviousID,
		analytics.WithContextOverride(analytics.AnalyticsContext{
			UserID:      req.Context.UserID,
			AnonymousID: req.Context.AnonymousID,
			Attributes:  req.Context.Attributes,
		})); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	var req BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Source == "" {
		jsonError(w, "source is required", http.StatusBadRequest)
		return
	}

	c, err := s.clientForSource(r.Context(), req.Source)
	if err != nil {
		sourceErrResponse(w, req.Source, err)
		return
	}

	for i, item := range req.Events {
		ec := mergeEventContext(req.Context, item.Context)
		ctxOpt := analytics.WithContextOverride(analytics.AnalyticsContext{
			UserID:      ec.UserID,
			AnonymousID: ec.AnonymousID,
			Attributes:  ec.Attributes,
		})

		var dispatchErr error
		switch item.Type {
		case "track":
			dispatchErr = c.Track(r.Context(), analytics.Event{Name: item.EventName, Properties: item.Properties}, ctxOpt)
		case "identify":
			dispatchErr = c.Identify(r.Context(), item.UserID, item.Traits, ctxOpt)
		case "group":
			dispatchErr = c.Group(r.Context(), item.GroupID, item.Traits, ctxOpt)
		case "page":
			dispatchErr = c.Page(r.Context(), item.Name, item.Properties, ctxOpt)
		case "alias":
			dispatchErr = c.Alias(r.Context(), item.UserID, item.PreviousID, ctxOpt)
		default:
			jsonError(w, fmt.Sprintf("events[%d]: unknown type %q", i, item.Type), http.StatusBadRequest)
			return
		}

		if dispatchErr != nil {
			jsonError(w, fmt.Sprintf("events[%d]: %s", i, dispatchErr), http.StatusBadRequest)
			return
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleFlush(w http.ResponseWriter, r *http.Request) {
	var req FlushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Source != "" {
		c, err := s.clientForSource(r.Context(), req.Source)
		if err != nil {
			sourceErrResponse(w, req.Source, err)
			return
		}
		if err := c.Flush(r.Context()); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		s.clientsMu.RLock()
		clients := make([]*analytics.Client, 0, len(s.clients))
		for _, c := range s.clients {
			clients = append(clients, c)
		}
		s.clientsMu.RUnlock()

		for _, c := range clients {
			if err := c.Flush(r.Context()); err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	w.WriteHeader(http.StatusAccepted)
}
