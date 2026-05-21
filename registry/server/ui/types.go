package ui

import (
	"context"
	"time"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/registry/server/shared"
	"github.com/dejanradmanovic/event-spec/spec"
)

// Role constant aliases — ui cannot import registry/server (would create a cycle
// because registry/server imports registry/server/ui). Both packages share the
// canonical values from registry/server/shared instead.
const (
	RoleViewer    = shared.RoleViewer
	RolePublisher = shared.RolePublisher
	RoleAdmin     = shared.RoleAdmin
)

const cookieName = "_esui"

type contextKey int

const (
	ctxUserID contextKey = iota
	ctxRole
)

// AuditFilter is an alias for shared.AuditFilter so handler files can use the short name.
// Using shared types ensures server.Store and ui.Store have identical method signatures —
// no adapter layer is required.
type AuditFilter = shared.AuditFilter

// AuditEntry is an alias for shared.AuditEntry.
type AuditEntry = shared.AuditEntry

// APIKeyRecord is an alias for shared.APIKeyRecord.
type APIKeyRecord = shared.APIKeyRecord

// WebhookRecord is an alias for shared.WebhookRecord.
type WebhookRecord = shared.WebhookRecord

// ServerSetting is an alias for shared.ServerSetting.
type ServerSetting = shared.ServerSetting

// Store is the persistence interface required by the UI handler.
// Because the method signatures use the same types as registry/server.Store
// (via shared), any value implementing registry/server.Store also implements
// this interface, so it can be passed directly without an adapter.
type Store interface {
	ListEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error)
	ListAllEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error)
	GetEvent(ctx context.Context, namespace, name, version string) (*spec.EventDef, error)
	PublishEvent(ctx context.Context, event spec.EventDef, userID string) error
	GetSource(ctx context.Context, name string) (*spec.SourceDef, error)
	ListSources(ctx context.Context) ([]spec.SourceDef, error)
	CreateSource(ctx context.Context, src spec.SourceDef, userID string) error
	UpdateSource(ctx context.Context, src spec.SourceDef, userID string) error
	DeleteSource(ctx context.Context, name string, userID string) error
	GetDestination(ctx context.Context, name string) (*spec.DestinationDef, error)
	ListDestinations(ctx context.Context) ([]string, error)
	ListDestinationsFull(ctx context.Context) ([]spec.DestinationDef, error)
	CreateDestination(ctx context.Context, dest spec.DestinationDef, userID string) error
	UpdateDestination(ctx context.Context, dest spec.DestinationDef, userID string) error
	DeleteDestination(ctx context.Context, name string, userID string) error
	CountAPIKeys(ctx context.Context) (int, error)
	LookupAPIKey(ctx context.Context, keyHash string) (userID, role string, err error)
	ListAuditLog(ctx context.Context, filter shared.AuditFilter) ([]shared.AuditEntry, error)
	ListAPIKeys(ctx context.Context) ([]shared.APIKeyRecord, error)
	CreateAPIKey(ctx context.Context, keyHash, role, name, createdBy string, expiresAt *time.Time) (int64, error)
	RevokeAPIKey(ctx context.Context, id int64) error
	ListWebhooksAdmin(ctx context.Context) ([]shared.WebhookRecord, error)
	RegisterWebhook(ctx context.Context, webhookURL, userID string) error
	DeleteWebhook(ctx context.Context, id int64) error
	ListSettings(ctx context.Context) ([]shared.ServerSetting, error)
	SetSetting(ctx context.Context, key, value string) error
}

// HooksEnabled returns the current live value of the hooks_enabled runtime toggle.
type HooksEnabled func() bool
