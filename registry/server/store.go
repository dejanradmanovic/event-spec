package server

import (
	"context"
	"time"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

// Store is the persistence layer for the Server.
// NewSQL provides a *sql.DB-backed implementation; custom implementations
// can be injected via New for testing or alternative backends.
type Store interface {
	// ListEvents returns one EventDef per (namespace, name) pair — the highest SchemaVer matching filter.
	ListEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error)
	// ListAllEvents returns every matching EventDef without deduplication.
	ListAllEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error)
	GetEvent(ctx context.Context, namespace, name, version string) (*spec.EventDef, error)
	GetSource(ctx context.Context, name string) (*spec.SourceDef, error)
	GetDestination(ctx context.Context, name string) (*spec.DestinationDef, error)
	ListDestinations(ctx context.Context) ([]string, error)
	// PublishEvent writes the event and records it in the audit log under userID.
	PublishEvent(ctx context.Context, event spec.EventDef, userID string) error
	// LookupAPIKey returns the user identity and role for the given SHA-256 key hash.
	// Returns registry.ErrNotFound when the key does not exist or has expired.
	LookupAPIKey(ctx context.Context, keyHash string) (userID, role string, err error)
	ListAuditLog(ctx context.Context, filter AuditFilter) ([]AuditEntry, error)
	RegisterWebhook(ctx context.Context, webhookURL, userID string) error
	// ListWebhooks returns all registered webhook URLs for event publish notifications.
	ListWebhooks(ctx context.Context) ([]string, error)

	// Admin: source management.
	ListSources(ctx context.Context) ([]spec.SourceDef, error)
	CreateSource(ctx context.Context, src spec.SourceDef, userID string) error
	UpdateSource(ctx context.Context, src spec.SourceDef, userID string) error
	DeleteSource(ctx context.Context, name string, userID string) error

	// Admin: destination management.
	ListDestinationsFull(ctx context.Context) ([]spec.DestinationDef, error)
	CreateDestination(ctx context.Context, dest spec.DestinationDef, userID string) error
	UpdateDestination(ctx context.Context, dest spec.DestinationDef, userID string) error
	DeleteDestination(ctx context.Context, name string, userID string) error

	// Admin: API key management.
	CountAPIKeys(ctx context.Context) (int, error)
	CreateAPIKey(ctx context.Context, keyHash, role, name, createdBy string, expiresAt *time.Time) (int64, error)
	ListAPIKeys(ctx context.Context) ([]APIKeyRecord, error)
	RevokeAPIKey(ctx context.Context, id int64) error

	// Admin: webhook management.
	ListWebhooksAdmin(ctx context.Context) ([]WebhookRecord, error)
	DeleteWebhook(ctx context.Context, id int64) error

	// Admin: runtime server settings (key-value store, persisted in DB).
	// GetSetting returns registry.ErrNotFound when the key has no stored value.
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	ListSettings(ctx context.Context) ([]ServerSetting, error)
}
