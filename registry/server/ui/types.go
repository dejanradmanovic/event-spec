package ui

import (
	"context"
	"time"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

// Role constants — the ui package cannot import registry/server (circular import),
// so these are defined here with the same values as server.Role*.
const (
	RoleViewer    = "viewer"
	RolePublisher = "publisher"
	RoleAdmin     = "admin"
)

const cookieName = "_esui"

type contextKey int

const (
	ctxUserID contextKey = iota
	ctxRole
)

// AuditFilter constrains an audit log query.
type AuditFilter struct {
	Since      *time.Time
	Until      *time.Time
	EntityType string
	UserID     string
	Limit      int
}

// AuditEntry is a single audit log record.
type AuditEntry struct {
	ID         int64
	Action     string
	EntityType string
	EntityID   int64
	UserID     string
	Timestamp  time.Time
	Details    string
}

// APIKeyRecord is the public metadata for a stored API key.
type APIKeyRecord struct {
	ID        int64
	Role      string
	Name      string
	CreatedBy string
	CreatedAt time.Time
	ExpiresAt *time.Time
}

// WebhookRecord is a registered webhook entry.
type WebhookRecord struct {
	ID        int64
	URL       string
	CreatedBy string
	CreatedAt time.Time
}

// ServerSetting is a runtime configuration key-value entry.
type ServerSetting struct {
	Key   string
	Value string
}

// Store is the persistence interface required by the UI handler.
type Store interface {
	ListEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error)
	GetEvent(ctx context.Context, namespace, name, version string) (*spec.EventDef, error)
	PublishEvent(ctx context.Context, event spec.EventDef, userID string) error
	LookupAPIKey(ctx context.Context, keyHash string) (userID, role string, err error)
	ListAuditLog(ctx context.Context, filter AuditFilter) ([]AuditEntry, error)
	ListAPIKeys(ctx context.Context) ([]APIKeyRecord, error)
	CreateAPIKey(ctx context.Context, keyHash, role, name, createdBy string, expiresAt *time.Time) (int64, error)
	RevokeAPIKey(ctx context.Context, id int64) error
	ListWebhooksAdmin(ctx context.Context) ([]WebhookRecord, error)
	RegisterWebhook(ctx context.Context, webhookURL, userID string) error
	DeleteWebhook(ctx context.Context, id int64) error
	ListSettings(ctx context.Context) ([]ServerSetting, error)
	SetSetting(ctx context.Context, key, value string) error
}

// HooksEnabled returns the current live value of the hooks_enabled runtime toggle.
type HooksEnabled func() bool
