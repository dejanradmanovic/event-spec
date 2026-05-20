package analytics

// Event is the canonical representation of an analytics event passed to Track.
type Event struct {
	Name       string
	Properties map[string]any
}
