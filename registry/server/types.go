package server

import (
	"time"

	"github.com/dejanradmanovic/event-spec/registry/server/shared"
	"github.com/dejanradmanovic/event-spec/spec"
)

// AuditFilter is an alias for shared.AuditFilter; defined here so existing server code needs no import change.
type AuditFilter = shared.AuditFilter

// AuditEntry is an alias for shared.AuditEntry.
type AuditEntry = shared.AuditEntry

// APIKeyRecord is an alias for shared.APIKeyRecord.
type APIKeyRecord = shared.APIKeyRecord

// WebhookRecord is an alias for shared.WebhookRecord.
type WebhookRecord = shared.WebhookRecord

// ServerSetting is an alias for shared.ServerSetting.
type ServerSetting = shared.ServerSetting

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
