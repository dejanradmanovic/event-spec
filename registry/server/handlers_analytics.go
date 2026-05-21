package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/dejanradmanovic/event-spec/analytics"
	"github.com/dejanradmanovic/event-spec/hooks/sampling"
	"github.com/dejanradmanovic/event-spec/hooks/validation"
	"github.com/dejanradmanovic/event-spec/provider"
	"github.com/dejanradmanovic/event-spec/provider/amplitude"
	"github.com/dejanradmanovic/event-spec/provider/noop"
	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

// clientForSource returns (or lazily builds) the analytics.Client for sourceName.
// The client is constructed from the source's destinations stored in the DB and cached
// for the lifetime of the server. Thread-safe via double-checked locking.
// Hooks (validation + per-event sampling) are always registered; the eventLookup
// function respects the hooksEnabled toggle so hook behaviour changes instantly
// without recreating cached clients.
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

	c = analytics.NewClient(
		analytics.WithProviders(ps...),
		analytics.WithHooks(
			validation.New(s.eventLookup, nil),
			sampling.New(s.eventLookup),
		),
	)
	s.clients[sourceName] = c
	return c, nil
}

// eventLookup satisfies the hooks.LookupFunc signature used by validation.Hook and sampling.Hook.
// Returns (nil, false) when hooks are disabled, so both hooks pass every event through unchanged.
// Results are cached in s.eventsByName and invalidated whenever a new event is published.
func (s *Server) eventLookup(eventName string) (*spec.EventDef, bool) {
	if !s.hooksEnabled.Load() {
		return nil, false
	}

	s.eventCacheMu.RLock()
	def, ready := s.eventsByName[eventName], s.eventCacheReady
	s.eventCacheMu.RUnlock()
	if ready {
		return def, def != nil
	}

	// Cache miss — populate from the store once, then return.
	events, err := s.st.ListEvents(context.Background(), registry.ListFilter{})
	if err != nil {
		return nil, false
	}

	s.eventCacheMu.Lock()
	defer s.eventCacheMu.Unlock()
	if s.eventCacheReady {
		def = s.eventsByName[eventName]
		return def, def != nil
	}
	s.eventsByName = make(map[string]*spec.EventDef, len(events))
	for i := range events {
		s.eventsByName[events[i].EventName] = &events[i]
	}
	s.eventCacheReady = true
	def = s.eventsByName[eventName]
	return def, def != nil
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

// enrichFromRequest fills in missing context attributes from the HTTP request.
// Client-supplied values always win; server-extracted values are used as fallbacks only.
// This ensures the MessageContext sent to providers carries real device/IP data even when
// thin clients (mobile, browser) omit attributes they expect the SDK to collect locally.
func enrichFromRequest(ec *EventContext, r *http.Request) {
	if ec.Attributes == nil {
		ec.Attributes = make(map[string]any)
	}
	if _, ok := ec.Attributes["user_agent"]; !ok {
		if ua := r.Header.Get("User-Agent"); ua != "" {
			ec.Attributes["user_agent"] = ua
		}
	}
	if _, ok := ec.Attributes["ip_address"]; !ok {
		if ip := extractClientIP(r); ip != "" {
			ec.Attributes["ip_address"] = ip
		}
	}
}

// extractClientIP returns the originating client IP, preferring X-Forwarded-For
// (the leftmost entry, set by the outermost reverse proxy) over RemoteAddr.
func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func sourceErrResponse(w http.ResponseWriter, sourceName string, err error) {
	if errors.Is(err, registry.ErrNotFound) {
		jsonError(w, "source not found: "+sourceName, http.StatusBadRequest)
	} else {
		jsonError(w, err.Error(), http.StatusInternalServerError)
	}
}

// dispatchErr converts a hook-cancelled error to the correct HTTP response.
// sampling.ErrSampled → 202 (silent drop); everything else → 400.
// Returns true if the handler should stop (response already written).
func writeSampledOrError(w http.ResponseWriter, err error, errCtx string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sampling.ErrSampled) {
		w.WriteHeader(http.StatusAccepted)
		return true
	}
	jsonError(w, errCtx+err.Error(), http.StatusBadRequest)
	return true
}

func (s *Server) handleTrack(w http.ResponseWriter, r *http.Request) {
	var req TrackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	enrichFromRequest(&req.Context, r)
	if req.Source == "" || req.EventName == "" {
		jsonError(w, "source and event_name are required", http.StatusBadRequest)
		return
	}

	c, err := s.clientForSource(r.Context(), req.Source)
	if err != nil {
		sourceErrResponse(w, req.Source, err)
		return
	}

	err = c.Track(r.Context(), analytics.Event{
		Name:       req.EventName,
		Properties: req.Properties,
	}, analytics.WithContextOverride(analytics.AnalyticsContext{
		UserID:      req.Context.UserID,
		AnonymousID: req.Context.AnonymousID,
		Attributes:  req.Context.Attributes,
	}))
	if writeSampledOrError(w, err, "") {
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
	enrichFromRequest(&req.Context, r)
	if req.Source == "" {
		jsonError(w, "source is required", http.StatusBadRequest)
		return
	}

	c, err := s.clientForSource(r.Context(), req.Source)
	if err != nil {
		sourceErrResponse(w, req.Source, err)
		return
	}

	err = c.Identify(r.Context(), req.UserID, req.Traits,
		analytics.WithContextOverride(analytics.AnalyticsContext{
			UserID:      req.Context.UserID,
			AnonymousID: req.Context.AnonymousID,
			Attributes:  req.Context.Attributes,
		}))
	if writeSampledOrError(w, err, "") {
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
	enrichFromRequest(&req.Context, r)
	if req.Source == "" {
		jsonError(w, "source is required", http.StatusBadRequest)
		return
	}

	c, err := s.clientForSource(r.Context(), req.Source)
	if err != nil {
		sourceErrResponse(w, req.Source, err)
		return
	}

	err = c.Group(r.Context(), req.GroupID, req.Traits,
		analytics.WithContextOverride(analytics.AnalyticsContext{
			UserID:      req.Context.UserID,
			AnonymousID: req.Context.AnonymousID,
			Attributes:  req.Context.Attributes,
		}))
	if writeSampledOrError(w, err, "") {
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
	enrichFromRequest(&req.Context, r)
	if req.Source == "" || req.Name == "" {
		jsonError(w, "source and name are required", http.StatusBadRequest)
		return
	}

	c, err := s.clientForSource(r.Context(), req.Source)
	if err != nil {
		sourceErrResponse(w, req.Source, err)
		return
	}

	err = c.Page(r.Context(), req.Name, req.Properties,
		analytics.WithContextOverride(analytics.AnalyticsContext{
			UserID:      req.Context.UserID,
			AnonymousID: req.Context.AnonymousID,
			Attributes:  req.Context.Attributes,
		}))
	if writeSampledOrError(w, err, "") {
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
	enrichFromRequest(&req.Context, r)
	if req.Source == "" {
		jsonError(w, "source is required", http.StatusBadRequest)
		return
	}

	c, err := s.clientForSource(r.Context(), req.Source)
	if err != nil {
		sourceErrResponse(w, req.Source, err)
		return
	}

	err = c.Alias(r.Context(), req.UserID, req.PreviousID,
		analytics.WithContextOverride(analytics.AnalyticsContext{
			UserID:      req.Context.UserID,
			AnonymousID: req.Context.AnonymousID,
			Attributes:  req.Context.Attributes,
		}))
	if writeSampledOrError(w, err, "") {
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
	enrichFromRequest(&req.Context, r)
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
			// Sampled-out items are silently skipped; other errors abort the batch.
			if errors.Is(dispatchErr, sampling.ErrSampled) {
				continue
			}
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
