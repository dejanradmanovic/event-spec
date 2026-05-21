package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

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
    name       TEXT,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS webhooks (
    id         BIGSERIAL PRIMARY KEY,
    url        TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS server_settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
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
    name       TEXT,
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at TEXT
);
CREATE TABLE IF NOT EXISTS webhooks (
    id         INTEGER PRIMARY KEY,
    url        TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS server_settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
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

// migrateAlter applies best-effort additive schema changes for databases created
// before the current DDL version. Errors are silently ignored — the column may
// already exist (SQLite) or use IF NOT EXISTS (PostgreSQL).
func (st *sqlStore) migrateAlter() {
	alters := []string{
		"ALTER TABLE api_keys ADD COLUMN name TEXT",
	}
	for _, stmt := range alters {
		_, _ = st.db.ExecContext(context.Background(), stmt)
	}
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

func (st *sqlStore) ListAllEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error) {
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

func (st *sqlStore) ListEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error) {
	all, err := st.ListAllEvents(ctx, filter)
	if err != nil {
		return nil, err
	}
	return registry.DeduplicateByLatest(all), nil
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

func (st *sqlStore) ListDestinationsFull(ctx context.Context) ([]spec.DestinationDef, error) {
	rows, err := st.db.QueryContext(ctx, "SELECT spec_yaml FROM destinations ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var dests []spec.DestinationDef
	for rows.Next() {
		var yamlData string
		if err := rows.Scan(&yamlData); err != nil {
			return nil, err
		}
		var def spec.DestinationDef
		if err := yaml.Unmarshal([]byte(yamlData), &def); err != nil {
			continue
		}
		dests = append(dests, def)
	}
	return dests, rows.Err()
}

func (st *sqlStore) CreateDestination(ctx context.Context, dest spec.DestinationDef, userID string) error {
	yamlData, err := yaml.Marshal(dest)
	if err != nil {
		return fmt.Errorf("marshal destination yaml: %w", err)
	}
	var destID int64
	if st.driver == "postgres" {
		q := `INSERT INTO destinations (name, spec_yaml) VALUES ($1, $2) RETURNING id`
		err = st.db.QueryRowContext(ctx, q, dest.Name, string(yamlData)).Scan(&destID)
	} else {
		q := `INSERT INTO destinations (name, spec_yaml) VALUES (?, ?)`
		var res sql.Result
		res, err = st.db.ExecContext(ctx, q, dest.Name, string(yamlData))
		if err == nil {
			destID, err = res.LastInsertId()
		}
	}
	if err != nil {
		return fmt.Errorf("insert destination: %w", err)
	}
	details := fmt.Sprintf(`{"name":%q,"provider":%q}`, dest.Name, dest.Provider)
	auditQ := st.ph("INSERT INTO audit_log (action, entity_type, entity_id, user_id, details) VALUES (?, 'destination', ?, ?, ?)")
	_, err = st.db.ExecContext(ctx, auditQ, "create", destID, userID, details)
	return err
}

func (st *sqlStore) UpdateDestination(ctx context.Context, dest spec.DestinationDef, userID string) error {
	yamlData, err := yaml.Marshal(dest)
	if err != nil {
		return fmt.Errorf("marshal destination yaml: %w", err)
	}
	var destID int64
	err = st.db.QueryRowContext(ctx, st.ph("SELECT id FROM destinations WHERE name = ?"), dest.Name).Scan(&destID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("destination %q: %w", dest.Name, registry.ErrNotFound)
	}
	if err != nil {
		return err
	}
	var q string
	if st.driver == "postgres" {
		q = "UPDATE destinations SET spec_yaml = $1, updated_at = NOW() WHERE name = $2"
	} else {
		q = "UPDATE destinations SET spec_yaml = ?, updated_at = datetime('now') WHERE name = ?"
	}
	if _, err = st.db.ExecContext(ctx, q, string(yamlData), dest.Name); err != nil {
		return fmt.Errorf("update destination: %w", err)
	}
	details := fmt.Sprintf(`{"name":%q,"provider":%q}`, dest.Name, dest.Provider)
	auditQ := st.ph("INSERT INTO audit_log (action, entity_type, entity_id, user_id, details) VALUES (?, 'destination', ?, ?, ?)")
	_, err = st.db.ExecContext(ctx, auditQ, "update", destID, userID, details)
	return err
}

func (st *sqlStore) DeleteDestination(ctx context.Context, name string, userID string) error {
	var destID int64
	_ = st.db.QueryRowContext(ctx, st.ph("SELECT id FROM destinations WHERE name = ?"), name).Scan(&destID)
	res, err := st.db.ExecContext(ctx, st.ph("DELETE FROM destinations WHERE name = ?"), name)
	if err != nil {
		return fmt.Errorf("delete destination: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("destination %q: %w", name, registry.ErrNotFound)
	}
	details := fmt.Sprintf(`{"name":%q}`, name)
	auditQ := st.ph("INSERT INTO audit_log (action, entity_type, entity_id, user_id, details) VALUES (?, 'destination', ?, ?, ?)")
	_, err = st.db.ExecContext(ctx, auditQ, "delete", destID, userID, details)
	return err
}

func (st *sqlStore) ListDestinations(ctx context.Context) ([]string, error) {
	rows, err := st.db.QueryContext(ctx, "SELECT name FROM destinations ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
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

func (st *sqlStore) ListAuditLog(ctx context.Context, filter AuditFilter) ([]AuditEntry, error) {
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

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

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
		if filter.EntityType != "" && e.EntityType != filter.EntityType {
			continue
		}
		if filter.UserID != "" && e.UserID != filter.UserID {
			continue
		}
		if filter.Since != nil && e.Timestamp.Before(*filter.Since) {
			continue
		}
		if filter.Until != nil && e.Timestamp.After(*filter.Until) {
			continue
		}
		entries = append(entries, e)
		if len(entries) >= limit {
			break
		}
	}
	_ = rows.Close()
	return entries, rows.Err()
}

func (st *sqlStore) CountAPIKeys(ctx context.Context) (int, error) {
	var n int
	err := st.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM api_keys").Scan(&n)
	return n, err
}

func (st *sqlStore) CreateAPIKey(ctx context.Context, keyHash, role, name, createdBy string, expiresAt *time.Time) (int64, error) {
	var expiresStr any
	if expiresAt != nil {
		expiresStr = expiresAt.UTC().Format(time.RFC3339)
	}
	if st.driver == "postgres" {
		q := `INSERT INTO api_keys (key_hash, role, name, created_by, expires_at) VALUES ($1, $2, $3, $4, $5) RETURNING id`
		var id int64
		err := st.db.QueryRowContext(ctx, q, keyHash, role, name, createdBy, expiresStr).Scan(&id)
		return id, err
	}
	q := `INSERT INTO api_keys (key_hash, role, name, created_by, expires_at) VALUES (?, ?, ?, ?, ?)`
	res, err := st.db.ExecContext(ctx, q, keyHash, role, name, createdBy, expiresStr)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (st *sqlStore) ListAPIKeys(ctx context.Context) ([]APIKeyRecord, error) {
	var q string
	if st.driver == "postgres" {
		q = `SELECT id, role, COALESCE(name, ''), created_by,
			to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			COALESCE(to_char(expires_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '')
			FROM api_keys ORDER BY id`
	} else {
		q = `SELECT id, role, COALESCE(name, ''), created_by, created_at, COALESCE(expires_at, '')
			FROM api_keys ORDER BY id`
	}
	rows, err := st.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var records []APIKeyRecord
	for rows.Next() {
		var r APIKeyRecord
		var createdTS, expiresTS string
		if err := rows.Scan(&r.ID, &r.Role, &r.Name, &r.CreatedBy, &createdTS, &expiresTS); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339, createdTS); err == nil {
			r.CreatedAt = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", createdTS); err == nil {
			r.CreatedAt = t
		}
		if expiresTS != "" {
			if t, err := time.Parse(time.RFC3339, expiresTS); err == nil {
				r.ExpiresAt = &t
			} else if t, err := time.Parse("2006-01-02 15:04:05", expiresTS); err == nil {
				r.ExpiresAt = &t
			}
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (st *sqlStore) RevokeAPIKey(ctx context.Context, id int64) error {
	_, err := st.db.ExecContext(ctx, st.ph("DELETE FROM api_keys WHERE id = ?"), id)
	return err
}

func (st *sqlStore) ListWebhooksAdmin(ctx context.Context) ([]WebhookRecord, error) {
	var q string
	if st.driver == "postgres" {
		q = `SELECT id, url, created_by,
			to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
			FROM webhooks ORDER BY id`
	} else {
		q = `SELECT id, url, created_by, created_at FROM webhooks ORDER BY id`
	}
	rows, err := st.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var records []WebhookRecord
	for rows.Next() {
		var r WebhookRecord
		var createdTS string
		if err := rows.Scan(&r.ID, &r.URL, &r.CreatedBy, &createdTS); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339, createdTS); err == nil {
			r.CreatedAt = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", createdTS); err == nil {
			r.CreatedAt = t
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (st *sqlStore) DeleteWebhook(ctx context.Context, id int64) error {
	_, err := st.db.ExecContext(ctx, st.ph("DELETE FROM webhooks WHERE id = ?"), id)
	return err
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

func (st *sqlStore) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := st.db.QueryRowContext(ctx, st.ph("SELECT value FROM server_settings WHERE key = ?"), key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", registry.ErrNotFound
	}
	return value, err
}

func (st *sqlStore) SetSetting(ctx context.Context, key, value string) error {
	var q string
	if st.driver == "postgres" {
		q = `INSERT INTO server_settings (key, value) VALUES ($1, $2)
             ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`
	} else {
		q = `INSERT INTO server_settings (key, value) VALUES (?, ?)
             ON CONFLICT(key) DO UPDATE SET value = excluded.value`
	}
	_, err := st.db.ExecContext(ctx, q, key, value)
	return err
}

func (st *sqlStore) ListSettings(ctx context.Context) ([]ServerSetting, error) {
	rows, err := st.db.QueryContext(ctx, "SELECT key, value FROM server_settings ORDER BY key")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var settings []ServerSetting
	for rows.Next() {
		var s ServerSetting
		if err := rows.Scan(&s.Key, &s.Value); err != nil {
			return nil, err
		}
		settings = append(settings, s)
	}
	return settings, rows.Err()
}
