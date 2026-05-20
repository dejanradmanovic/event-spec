package analytics

// Event status constants mirror spec.EventStatus but are re-exported here so
// generated code only needs to import the analytics package.
const (
	EventStatusDraft      = "draft"
	EventStatusActive     = "active"
	EventStatusDeprecated = "deprecated"
	EventStatusDeleted    = "deleted"
)

// Event is the canonical representation of an analytics event passed to Track.
type Event struct {
	Name       string
	Properties map[string]any
	// Status is the lifecycle state from the event spec.
	// Generated wrappers for draft events set this to EventStatusDraft.
	// Empty is treated as active.
	Status string
}
