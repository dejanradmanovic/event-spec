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
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

// Role constants for API key authorization.
const (
	RoleViewer    = "viewer"
	RolePublisher = "publisher"
	RoleAdmin     = "admin"
)

// contextKey is an unexported type for context values to avoid collisions.
type contextKey int

const (
	ctxUserID contextKey = iota
	ctxRole
)

// Config configures the registry server.
type Config struct {
	// Port is the HTTP listen port. Defaults to 8080 when zero.
	Port int
}

// AuditEntry is a single record from the audit log.
type AuditEntry struct {
	ID         int64     `json:"id"`
	Action     string    `json:"action"`      // "create" | "update"
	EntityType string    `json:"entity_type"` // "event" | "source" | "destination"
	EntityID   int64     `json:"entity_id"`
	UserID     string    `json:"user_id"`
	Timestamp  time.Time `json:"timestamp"`
	Details    string    `json:"details,omitempty"`
}

// WebhookPayload is the JSON body sent to each registered webhook when an event is published.
type WebhookPayload struct {
	Event       spec.EventDef `json:"event"`
	PublishedBy string        `json:"published_by"`
}

// Store is the persistence layer for the Server.
// NewSQL provides a *sql.DB-backed implementation; custom implementations
// can be injected via New for testing or alternative backends.
type Store interface {
	ListEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error)
	GetEvent(ctx context.Context, namespace, name, version string) (*spec.EventDef, error)
	GetSource(ctx context.Context, name string) (*spec.SourceDef, error)
	GetDestination(ctx context.Context, name string) (*spec.DestinationDef, error)
	// PublishEvent writes the event and records it in the audit log under userID.
	PublishEvent(ctx context.Context, event spec.EventDef, userID string) error
	// LookupAPIKey returns the user identity and role for the given SHA-256 key hash.
	// Returns registry.ErrNotFound when the key does not exist or has expired.
	LookupAPIKey(ctx context.Context, keyHash string) (userID, role string, err error)
	ListAuditLog(ctx context.Context) ([]AuditEntry, error)
	RegisterWebhook(ctx context.Context, webhookURL, userID string) error
	// ListWebhooks returns all registered webhook URLs for event publish notifications.
	ListWebhooks(ctx context.Context) ([]string, error)
}

// Server is the REST API registry server.
// It implements http.Handler for serving the REST API and registry.Registry for
// in-process access (e.g. embedded deployments and tests).
type Server struct {
	st  Store
	cfg Config
	mux *http.ServeMux
}

// New creates a Server backed by st.
// Use NewSQL or NewFromDSN for production SQL-backed servers.
func New(st Store, cfg Config) *Server {
	if cfg.Port <= 0 {
		cfg.Port = 8080
	}
	s := &Server{st: st, cfg: cfg, mux: http.NewServeMux()}
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
func (s *Server) PublishEvent(ctx context.Context, event spec.EventDef) error {
	userID, _ := ctx.Value(ctxUserID).(string)
	return s.st.PublishEvent(ctx, event, userID)
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

// --- auth helpers ---

func sha256hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func roleLevel(role string) int {
	switch role {
	case RoleAdmin:
		return 2
	case RolePublisher:
		return 1
	default: // viewer or unknown
		return 0
	}
}

// --- sqlStore ---

type sqlStore struct {
	db     *sql.DB
	driver string // "postgres" or "sqlite"
}

// ph converts ? placeholders to $N for PostgreSQL.
func (st *sqlStore) ph(query string) string {
	if st.driver != "postgres" {
		return query
	}
	n := 0
	var b strings.Builder
	for _, c := range query {
		if c == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

const postgresDDL = `
CREATE TABLE IF NOT EXISTS events (
    id         BIGSERIAL PRIMARY KEY,
    namespace  TEXT NOT NULL,
    name       TEXT NOT NULL,
    version    TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'draft',
    spec_yaml  TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    changelog  TEXT,
    UNIQUE (namespace, name, version)
);
CREATE TABLE IF NOT EXISTS sources (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT UNIQUE NOT NULL,
    spec_yaml  TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS destinations (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT UNIQUE NOT NULL,
    spec_yaml  TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS audit_log (
    id          BIGSERIAL PRIMARY KEY,
    action      TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id   BIGINT NOT NULL,
    user_id     TEXT NOT NULL,
    timestamp   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    details     TEXT
);
CREATE TABLE IF NOT EXISTS api_keys (
    id         BIGSERIAL PRIMARY KEY,
    key_hash   TEXT NOT NULL UNIQUE,
    role       TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS webhooks (
    id         BIGSERIAL PRIMARY KEY,
    url        TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
`

const sqliteDDL = `
CREATE TABLE IF NOT EXISTS events (
    id        INTEGER PRIMARY KEY,
    namespace TEXT NOT NULL,
    name      TEXT NOT NULL,
    version   TEXT NOT NULL,
    status    TEXT NOT NULL DEFAULT 'draft',
    spec_yaml TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    changelog TEXT,
    UNIQUE (namespace, name, version)
);
CREATE TABLE IF NOT EXISTS sources (
    id         INTEGER PRIMARY KEY,
    name       TEXT UNIQUE NOT NULL,
    spec_yaml  TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS destinations (
    id         INTEGER PRIMARY KEY,
    name       TEXT UNIQUE NOT NULL,
    spec_yaml  TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS audit_log (
    id          INTEGER PRIMARY KEY,
    action      TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id   INTEGER NOT NULL,
    user_id     TEXT NOT NULL,
    timestamp   TEXT NOT NULL DEFAULT (datetime('now')),
    details     TEXT
);
CREATE TABLE IF NOT EXISTS api_keys (
    id         INTEGER PRIMARY KEY,
    key_hash   TEXT NOT NULL UNIQUE,
    role       TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at TEXT
);
CREATE TABLE IF NOT EXISTS webhooks (
    id         INTEGER PRIMARY KEY,
    url        TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
)
`

func (st *sqlStore) migrate() error {
	ddl := postgresDDL
	if st.driver != "postgres" {
		ddl = sqliteDDL
	}
	for _, stmt := range splitStatements(ddl) {
		if _, err := st.db.ExecContext(context.Background(), stmt); err != nil {
			preview := stmt
			if len(preview) > 60 {
				preview = preview[:60]
			}
			return fmt.Errorf("exec %q: %w", preview, err)
		}
	}
	return nil
}

func splitStatements(s string) []string {
	parts := strings.Split(s, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (st *sqlStore) ListEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error) {
	query := "SELECT spec_yaml FROM events WHERE 1=1"
	var args []any
	if filter.Namespace != "" {
		query += " AND namespace = ?"
		args = append(args, filter.Namespace)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}
	rows, err := st.db.QueryContext(ctx, st.ph(query), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []spec.EventDef
	for rows.Next() {
		var yamlData string
		if err := rows.Scan(&yamlData); err != nil {
			return nil, err
		}
		var def spec.EventDef
		if err := yaml.Unmarshal([]byte(yamlData), &def); err != nil {
			continue // skip corrupted rows
		}
		if !containsAll(def.Tags, filter.Tags) {
			continue
		}
		results = append(results, def)
	}
	return results, rows.Err()
}

func containsAll(tags, required []string) bool {
	if len(required) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		set[t] = struct{}{}
	}
	for _, req := range required {
		if _, ok := set[req]; !ok {
			return false
		}
	}
	return true
}

func (st *sqlStore) GetEvent(ctx context.Context, namespace, name, version string) (*spec.EventDef, error) {
	if version != "" {
		var yamlData string
		q := st.ph("SELECT spec_yaml FROM events WHERE namespace = ? AND name = ? AND version = ?")
		err := st.db.QueryRowContext(ctx, q, namespace, name, version).Scan(&yamlData)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("event %s/%s@%s: %w", namespace, name, version, registry.ErrNotFound)
		}
		if err != nil {
			return nil, err
		}
		var def spec.EventDef
		if err := yaml.Unmarshal([]byte(yamlData), &def); err != nil {
			return nil, fmt.Errorf("parse event yaml: %w", err)
		}
		return &def, nil
	}

	// No version specified: fetch all active versions and pick the highest SchemaVer.
	q := st.ph("SELECT spec_yaml FROM events WHERE namespace = ? AND name = ? AND status = 'active'")
	rows, err := st.db.QueryContext(ctx, q, namespace, name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var best *spec.EventDef
	var bestVer spec.SchemaVer
	for rows.Next() {
		var yamlData string
		if err := rows.Scan(&yamlData); err != nil {
			return nil, err
		}
		var def spec.EventDef
		if err := yaml.Unmarshal([]byte(yamlData), &def); err != nil {
			continue
		}
		sv, err := spec.ParseSchemaVer(def.Version)
		if err != nil {
			continue
		}
		if best == nil || spec.CompareSchemaVer(sv, bestVer) > 0 {
			d := def
			best = &d
			bestVer = sv
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if best == nil {
		return nil, fmt.Errorf("event %s/%s (active): %w", namespace, name, registry.ErrNotFound)
	}
	return best, nil
}

func (st *sqlStore) GetSource(ctx context.Context, name string) (*spec.SourceDef, error) {
	var yamlData string
	q := st.ph("SELECT spec_yaml FROM sources WHERE name = ?")
	err := st.db.QueryRowContext(ctx, q, name).Scan(&yamlData)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("source %q: %w", name, registry.ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	var def spec.SourceDef
	if err := yaml.Unmarshal([]byte(yamlData), &def); err != nil {
		return nil, fmt.Errorf("parse source yaml: %w", err)
	}
	return &def, nil
}

func (st *sqlStore) GetDestination(ctx context.Context, name string) (*spec.DestinationDef, error) {
	var yamlData string
	q := st.ph("SELECT spec_yaml FROM destinations WHERE name = ?")
	err := st.db.QueryRowContext(ctx, q, name).Scan(&yamlData)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("destination %q: %w", name, registry.ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	var def spec.DestinationDef
	if err := yaml.Unmarshal([]byte(yamlData), &def); err != nil {
		return nil, fmt.Errorf("parse destination yaml: %w", err)
	}
	return &def, nil
}

func (st *sqlStore) PublishEvent(ctx context.Context, event spec.EventDef, userID string) error {
	if event.Status == "" {
		event.Status = spec.StatusDraft
	}
	yamlData, err := yaml.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event yaml: %w", err)
	}

	var eventID int64
	if st.driver == "postgres" {
		q := `INSERT INTO events (namespace, name, version, status, spec_yaml, changelog)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (namespace, name, version) DO UPDATE
			SET status = EXCLUDED.status, spec_yaml = EXCLUDED.spec_yaml, changelog = EXCLUDED.changelog
			RETURNING id`
		err = st.db.QueryRowContext(ctx, q,
			event.Namespace, event.Name, event.Version,
			string(event.Status), string(yamlData), event.Changelog).Scan(&eventID)
	} else {
		q := `INSERT INTO events (namespace, name, version, status, spec_yaml, changelog)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (namespace, name, version) DO UPDATE
			SET status = excluded.status, spec_yaml = excluded.spec_yaml, changelog = excluded.changelog`
		var res sql.Result
		res, err = st.db.ExecContext(ctx, q,
			event.Namespace, event.Name, event.Version,
			string(event.Status), string(yamlData), event.Changelog)
		if err == nil {
			eventID, err = res.LastInsertId()
		}
	}
	if err != nil {
		return fmt.Errorf("upsert event: %w", err)
	}

	details := fmt.Sprintf(`{"namespace":%q,"name":%q,"version":%q}`, event.Namespace, event.Name, event.Version)
	auditQ := st.ph("INSERT INTO audit_log (action, entity_type, entity_id, user_id, details) VALUES (?, 'event', ?, ?, ?)")
	_, err = st.db.ExecContext(ctx, auditQ, "create", eventID, userID, details)
	return err
}

func (st *sqlStore) LookupAPIKey(ctx context.Context, keyHash string) (userID, role string, err error) {
	var expiresAt sql.NullString
	q := st.ph("SELECT created_by, role, expires_at FROM api_keys WHERE key_hash = ?")
	err = st.db.QueryRowContext(ctx, q, keyHash).Scan(&userID, &role, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", registry.ErrNotFound
	}
	if err != nil {
		return "", "", err
	}
	if expiresAt.Valid && expiresAt.String != "" {
		if exp, parseErr := time.Parse(time.RFC3339, expiresAt.String); parseErr == nil && exp.Before(time.Now()) {
			return "", "", registry.ErrNotFound
		}
	}
	return userID, role, nil
}

func (st *sqlStore) ListAuditLog(ctx context.Context) ([]AuditEntry, error) {
	var q string
	if st.driver == "postgres" {
		q = `SELECT id, action, entity_type, entity_id, user_id,
			to_char(timestamp AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			COALESCE(details, '') FROM audit_log ORDER BY id DESC LIMIT 1000`
	} else {
		q = `SELECT id, action, entity_type, entity_id, user_id, timestamp, COALESCE(details, '')
			FROM audit_log ORDER BY id DESC LIMIT 1000`
	}
	rows, err := st.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var ts string
		if err := rows.Scan(&e.ID, &e.Action, &e.EntityType, &e.EntityID, &e.UserID, &ts, &e.Details); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			e.Timestamp = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", ts); err == nil {
			e.Timestamp = t
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (st *sqlStore) RegisterWebhook(ctx context.Context, webhookURL, userID string) error {
	q := st.ph("INSERT INTO webhooks (url, created_by) VALUES (?, ?)")
	_, err := st.db.ExecContext(ctx, q, webhookURL, userID)
	return err
}

func (st *sqlStore) ListWebhooks(ctx context.Context) ([]string, error) {
	rows, err := st.db.QueryContext(ctx, "SELECT url FROM webhooks")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var urls []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		urls = append(urls, u)
	}
	return urls, rows.Err()
}
