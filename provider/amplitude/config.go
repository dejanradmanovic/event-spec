// Package amplitude implements an Amplitude analytics provider using the batch HTTP API.
package amplitude

import "github.com/dejanradmanovic/event-spec/provider"

const (
	// defaultEndpoint is the Amplitude batch ingest endpoint.
	defaultEndpoint = "https://api2.amplitude.com/batch"

	version = "0.1.0"
)

// Config holds Amplitude provider settings.
// All batching, retry, proxy, rate-limit, and secret settings live in the
// embedded ProviderConfig; Endpoint is the only Amplitude-specific field.
type Config struct {
	provider.ProviderConfig

	// Endpoint overrides the Amplitude batch API URL.
	// Defaults to https://api2.amplitude.com/batch.
	Endpoint string
}
