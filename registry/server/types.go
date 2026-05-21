package server

import (
	"time"

	"github.com/dejanradmanovic/event-spec/spec"
)

// AuditFilter constrains an audit log query. Zero values mean no restriction.
type AuditFilter struct {
	Since      *time.Time
	Until      *time.Time
	EntityType string // "event" | "source" | "destination"
	UserID     string
	Limit      int // 0 means default (50)
}

// APIKeyRecord is the public metadata for a stored API key (never includes the raw key).
type APIKeyRecord struct {
	ID        int64      `json:"id"`
	Role      string     `json:"role"`
	Name      string     `json:"name,omitempty"`
	CreatedBy string     `json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// WebhookRecord is a registered webhook entry with its database ID.
type WebhookRecord struct {
	ID        int64     `json:"id"`
	URL       string    `json:"url"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
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

// ServerSetting is a single runtime configuration entry stored in the database.
type ServerSetting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// WebhookPayload is the JSON body sent to each registered webhook when an event is published.
type WebhookPayload struct {
	Event       spec.EventDef `json:"event"`
	PublishedBy string        `json:"published_by"`
}

// EventContext carries the user/session identity forwarded by a thin client.
type EventContext struct {
	UserID      string         `json:"user_id,omitempty"`
	AnonymousID string         `json:"anonymous_id,omitempty"`
	Attributes  map[string]any `json:"attributes,omitempty"`
}

// TrackRequest is the body for POST /v1/track.
type TrackRequest struct {
	Source     string         `json:"source"`
	EventName  string         `json:"event_name"`
	Properties map[string]any `json:"properties,omitempty"`
	Context    EventContext   `json:"context,omitempty"`
	Timestamp  time.Time      `json:"timestamp,omitempty"`
}

// IdentifyRequest is the body for POST /v1/identify.
type IdentifyRequest struct {
	Source    string         `json:"source"`
	UserID    string         `json:"user_id"`
	Traits    map[string]any `json:"traits,omitempty"`
	Context   EventContext   `json:"context,omitempty"`
	Timestamp time.Time      `json:"timestamp,omitempty"`
}

// GroupRequest is the body for POST /v1/group.
type GroupRequest struct {
	Source    string         `json:"source"`
	GroupID   string         `json:"group_id"`
	Traits    map[string]any `json:"traits,omitempty"`
	Context   EventContext   `json:"context,omitempty"`
	Timestamp time.Time      `json:"timestamp,omitempty"`
}

// PageRequest is the body for POST /v1/page.
type PageRequest struct {
	Source     string         `json:"source"`
	Name       string         `json:"name"`
	Properties map[string]any `json:"properties,omitempty"`
	Context    EventContext   `json:"context,omitempty"`
	Timestamp  time.Time      `json:"timestamp,omitempty"`
}

// AliasRequest is the body for POST /v1/alias.
type AliasRequest struct {
	Source     string       `json:"source"`
	UserID     string       `json:"user_id"`
	PreviousID string       `json:"previous_id"`
	Context    EventContext `json:"context,omitempty"`
	Timestamp  time.Time    `json:"timestamp,omitempty"`
}

// BatchItem is a single event within a BatchRequest.
// The Type field selects the analytics method: "track" | "identify" | "group" | "page" | "alias".
type BatchItem struct {
	Type       string         `json:"type"`
	EventName  string         `json:"event_name,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
	Traits     map[string]any `json:"traits,omitempty"`
	UserID     string         `json:"user_id,omitempty"`
	PreviousID string         `json:"previous_id,omitempty"`
	GroupID    string         `json:"group_id,omitempty"`
	Name       string         `json:"name,omitempty"`
	Context    EventContext   `json:"context,omitempty"`
	Timestamp  time.Time      `json:"timestamp,omitempty"`
}

// BatchRequest is the body for POST /v1/batch.
// Context provides a default identity for all items; per-item Context overrides take precedence.
type BatchRequest struct {
	Source  string       `json:"source"`
	Context EventContext `json:"context,omitempty"`
	Events  []BatchItem  `json:"events"`
}

// FlushRequest is the optional body for POST /v1/flush.
// An empty Source flushes all cached per-source clients.
type FlushRequest struct {
	Source string `json:"source,omitempty"`
}
