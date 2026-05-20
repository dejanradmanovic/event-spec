// Package registry defines the Registry interface for accessing event specs,
// sources, and destinations. Both the git-backed and server implementations
// satisfy this interface so callers remain agnostic to the storage backend.
package registry

import (
	"context"
	"errors"

	"event-spec/spec"
)

// ErrReadOnly is returned by PublishEvent on git-backed registries.
// Use git commits to publish new event spec versions in git mode.
var ErrReadOnly = errors.New("registry is read-only; use git commits to publish event specs")

// ErrNotFound is returned when a requested resource does not exist in the registry.
var ErrNotFound = errors.New("not found")

// ListFilter constrains the events returned by Registry.ListEvents.
// Zero-value fields are ignored (no filtering on that dimension).
type ListFilter struct {
	Namespace string           // restrict to this namespace; empty means all namespaces
	Status    spec.EventStatus // restrict to this status; empty means all statuses
	Tags      []string         // all listed tags must be present; nil means no tag filter
}

// Registry provides access to event specs, sources, and destinations.
// Both the git-backed and server implementations satisfy this interface;
// the CLI and codegen engine never import a concrete implementation directly.
type Registry interface {
	ListEvents(ctx context.Context, filter ListFilter) ([]spec.EventDef, error)
	GetEvent(ctx context.Context, namespace, name, version string) (*spec.EventDef, error)
	GetSource(ctx context.Context, name string) (*spec.SourceDef, error)
	GetDestination(ctx context.Context, name string) (*spec.DestinationDef, error)
	// PublishEvent writes a new event version. Returns ErrReadOnly in git mode.
	PublishEvent(ctx context.Context, event spec.EventDef) error
	// Diff returns the detected changes between two versions of an event.
	// Full computation is implemented alongside the event-spec diff CLI command.
	Diff(ctx context.Context, namespace, name, from, to string) ([]spec.Change, error)
}
