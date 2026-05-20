package analytics

import "time"

// DeliveryState is the outcome of dispatching an event to a single provider.
type DeliveryState string

const (
	// StateAccepted means the event entered the provider queue (async/batched).
	StateAccepted DeliveryState = "accepted"
	// StateDelivered means the provider confirmed receipt over the network.
	StateDelivered DeliveryState = "delivered"
	// StateFailed means the provider permanently rejected the event after max retries.
	StateFailed DeliveryState = "failed"
	// StateDropped means the event was intentionally discarded (sampling, consent, overflow).
	StateDropped DeliveryState = "dropped"
)

// ProviderResult is the per-provider outcome for a single event dispatch.
type ProviderResult struct {
	ProviderName string
	State        DeliveryState
	Error        error
	Latency      time.Duration
}

// DispatchResult aggregates per-provider outcomes for a single event dispatch.
// Returned by TrackDetailed for callers that need partial-failure visibility.
type DispatchResult struct {
	Success        []ProviderResult // providers that succeeded
	Failed         []ProviderResult // providers that permanently failed
	PartialSuccess bool             // true if at least one provider succeeded
}
