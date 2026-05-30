// Package eventspec implements a thin-client analytics provider that forwards
// events to an event-spec runtime ingestion server over HTTP.
package eventspec

import "github.com/dejanradmanovic/event-spec/provider"

const version = "1.0.0"

// Config holds event-spec server provider settings.
// BaseURL, APIKey, and Source are the only provider-specific fields;
// all retry, proxy, and rate-limit settings live in the embedded ProviderConfig.
type Config struct {
	provider.ProviderConfig

	// BaseURL is the event-spec server root (e.g. "https://events.internal").
	BaseURL string

	// APIKey is the bearer token sent in the Authorization header.
	// Resolved according to ProviderConfig.SecretType.
	APIKey string

	// Source identifies the calling application (e.g. "web-app").
	Source string
}
