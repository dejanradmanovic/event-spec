-- PostgreSQL schema for the event-spec registry server.
-- For SQLite, the server package applies an equivalent dialect automatically.

CREATE TABLE IF NOT EXISTS events (
    id         BIGSERIAL PRIMARY KEY,
    namespace  TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    version    TEXT        NOT NULL,
    status     TEXT        NOT NULL DEFAULT 'draft',
    spec_yaml  TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    changelog  TEXT,
    UNIQUE (namespace, name, version)
);

CREATE TABLE IF NOT EXISTS sources (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT        UNIQUE NOT NULL,
    spec_yaml  TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS destinations (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT        UNIQUE NOT NULL,
    spec_yaml  TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- audit_log records every spec change with the identity of the actor.
CREATE TABLE IF NOT EXISTS audit_log (
    id          BIGSERIAL   PRIMARY KEY,
    action      TEXT        NOT NULL,  -- 'create' | 'update'
    entity_type TEXT        NOT NULL,  -- 'event' | 'source' | 'destination'
    entity_id   BIGINT      NOT NULL,
    user_id     TEXT        NOT NULL,
    timestamp   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    details     TEXT                   -- JSON blob with change metadata
);

-- api_keys stores SHA-256 hashes of Bearer tokens.
-- Raw tokens are shown once at creation; only the hash is persisted.
CREATE TABLE IF NOT EXISTS api_keys (
    id         BIGSERIAL   PRIMARY KEY,
    key_hash   TEXT        NOT NULL UNIQUE,
    role       TEXT        NOT NULL,  -- 'viewer' | 'publisher' | 'admin'
    created_by TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ           -- NULL means no expiry
);

CREATE TABLE IF NOT EXISTS webhooks (
    id         BIGSERIAL   PRIMARY KEY,
    url        TEXT        NOT NULL,
    created_by TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
