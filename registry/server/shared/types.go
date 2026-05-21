// Package shared holds types and helpers shared between registry/server and
// registry/server/ui. Neither package should duplicate these definitions;
// both import from here instead.
package shared

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Role constants for API key authorisation.
const (
	RoleViewer    = "viewer"
	RolePublisher = "publisher"
	RoleAdmin     = "admin"
)

// Sha256Hex returns the lower-case hex-encoded SHA-256 hash of s.
func Sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// RoleLevel maps a role name to a numeric level used for >= comparisons.
// Unknown roles return 0 (same as viewer).
func RoleLevel(role string) int {
	switch role {
	case RoleAdmin:
		return 2
	case RolePublisher:
		return 1
	default:
		return 0
	}
}

// AuditFilter constrains an audit log query. Zero values mean no restriction.
type AuditFilter struct {
	Since      *time.Time
	Until      *time.Time
	EntityType string // "event" | "source" | "destination"
	UserID     string
	Limit      int // 0 means default (50)
}

// AuditEntry is a single record from the audit log.
type AuditEntry struct {
	ID         int64
	Action     string
	EntityType string
	EntityID   int64
	UserID     string
	Timestamp  time.Time
	Details    string
}

// APIKeyRecord is the public metadata for a stored API key (never includes the raw key).
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
