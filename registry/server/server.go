// Package server implements a REST API registry server backed by PostgreSQL or SQLite.
// It satisfies the registry.Registry interface so it can be used interchangeably with
// the local and git-backed registry implementations.
//
// Driver registration: the caller must blank-import a database driver before calling
// NewFromDSN. For PostgreSQL use github.com/lib/pq; for SQLite use modernc.org/sqlite
// or github.com/mattn/go-sqlite3.
package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dejanradmanovic/event-spec/analytics"
	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

// Config configures the registry server.
type Config struct {
	// Port is the HTTP listen port. Defaults to 8080 when zero.
	Port int
	// HooksDisabled turns off the validation and per-event sampling hooks on analytics
	// relay endpoints. By default (zero value) hooks are enabled: the server applies
	// validation.Hook (schema checks, deleted-event gates) and sampling.Hook (per-event
	// SamplingConfig from the spec) to every relay call. The runtime value can be
	// overridden at any time via the PUT /v1/admin/config/hooks_enabled endpoint without
	// a restart.
	HooksDisabled bool
}

// Server is the REST API registry server.
// It implements http.Handler for serving the REST API and registry.Registry for
// in-process access (e.g. embedded deployments and tests).
type Server struct {
	st        Store
	cfg       Config
	mux       *http.ServeMux
	startedAt time.Time

	clientsMu sync.RWMutex
	clients   map[string]*analytics.Client // analytics clients keyed by source name

	// hooksEnabled is the live toggle for analytics relay hooks.
	// Reads/writes use atomic operations so the admin endpoint can flip the flag
	// without locking or restarting clients.
	hooksEnabled atomic.Bool

	// eventsByName is a lazily-populated, write-invalidated cache of EventName → EventDef.
	// Used by eventLookup to avoid a DB round-trip on every relay call.
	eventCacheMu    sync.RWMutex
	eventCacheReady bool
	eventsByName    map[string]*spec.EventDef
}

// New creates a Server backed by st.
// Use NewSQL or NewFromDSN for production SQL-backed servers.
func New(st Store, cfg Config) *Server {
	if cfg.Port <= 0 {
		cfg.Port = 8080
	}
	s := &Server{
		st:        st,
		cfg:       cfg,
		mux:       http.NewServeMux(),
		startedAt: time.Now(),
		clients:   make(map[string]*analytics.Client),
	}
	s.hooksEnabled.Store(!cfg.HooksDisabled)

	// DB setting overrides the startup Config value so the admin endpoint
	// can toggle hooks without a server restart.
	if val, err := st.GetSetting(context.Background(), "hooks_enabled"); err == nil {
		s.hooksEnabled.Store(val == "true")
	}

	s.routes()
	return s
}

// NewSQL creates a Server backed by the given *sql.DB.
// driver must be "postgres" or "sqlite"; the schema is migrated automatically.
// The caller must blank-import the appropriate driver package before calling sql.Open.
func NewSQL(db *sql.DB, driver string, cfg Config) (*Server, error) {
	st := &sqlStore{db: db, driver: driver}
	if err := st.migrate(); err != nil {
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	st.migrateAlter() // best-effort additive column migrations
	return New(st, cfg), nil
}

// NewFromDSN opens a database from dsn (driver detected from the DSN prefix) and
// creates a Server. The caller must blank-import the required driver package:
//
//   - PostgreSQL (prefix postgres:// or postgresql:// or "host="): github.com/lib/pq
//   - SQLite (any other value): modernc.org/sqlite or github.com/mattn/go-sqlite3
func NewFromDSN(dsn string, cfg Config) (*Server, error) {
	driver := detectDriver(dsn)
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	return NewSQL(db, driver, cfg)
}

func detectDriver(dsn string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") || strings.HasPrefix(dsn, "host=") {
		return "postgres"
	}
	return "sqlite"
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server on the configured port.
func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	return http.ListenAndServe(addr, s)
}

// --- registry.Registry implementation ---

// ListEvents implements registry.Registry.
func (s *Server) ListEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error) {
	return s.st.ListEvents(ctx, filter)
}

// ListAllEvents implements registry.Registry.
func (s *Server) ListAllEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error) {
	return s.st.ListAllEvents(ctx, filter)
}

// GetEvent implements registry.Registry.
func (s *Server) GetEvent(ctx context.Context, namespace, name, version string) (*spec.EventDef, error) {
	return s.st.GetEvent(ctx, namespace, name, version)
}

// GetSource implements registry.Registry.
func (s *Server) GetSource(ctx context.Context, name string) (*spec.SourceDef, error) {
	return s.st.GetSource(ctx, name)
}

// GetDestination implements registry.Registry.
func (s *Server) GetDestination(ctx context.Context, name string) (*spec.DestinationDef, error) {
	return s.st.GetDestination(ctx, name)
}

// PublishEvent implements registry.Registry.
// The userID is extracted from ctx (populated by the auth middleware for HTTP requests).
// Invalidates the event name cache so hook lookups pick up the new definition immediately.
func (s *Server) PublishEvent(ctx context.Context, event spec.EventDef) error {
	userID, _ := ctx.Value(ctxUserID).(string)
	err := s.st.PublishEvent(ctx, event, userID)
	if err == nil {
		s.eventCacheMu.Lock()
		s.eventCacheReady = false
		s.eventsByName = nil
		s.eventCacheMu.Unlock()
	}
	return err
}

// Diff implements registry.Registry.
func (s *Server) Diff(ctx context.Context, namespace, name, from, to string) ([]spec.Change, error) {
	fromDef, err := s.st.GetEvent(ctx, namespace, name, from)
	if err != nil {
		return nil, fmt.Errorf("get version %s: %w", from, err)
	}
	toDef, err := s.st.GetEvent(ctx, namespace, name, to)
	if err != nil {
		return nil, fmt.Errorf("get version %s: %w", to, err)
	}
	return spec.Diff(fromDef, toDef), nil
}
