package provider

import (
	"crypto/tls"
	"time"
)

// SecretType controls how the provider resolves its API key.
type SecretType string

const (
	SecretEnvVar SecretType = "env_var" // read from environment variable
	SecretFile   SecretType = "file"    // read from file path
	SecretVault  SecretType = "vault"   // fetch from HashiCorp Vault, AWS Secrets Manager, etc.
	SecretInline SecretType = "inline"  // plaintext in config (dev only)
)

// ProxyMode controls how HTTP traffic is routed to the provider's API.
type ProxyMode string

const (
	ProxyDirect       ProxyMode = "direct"        // no proxy
	ProxyReverseProxy ProxyMode = "reverse_proxy" // send to your domain, proxy to provider
	ProxyCustom       ProxyMode = "custom"        // custom URL rewrite
)

// OverflowPolicy controls what happens when the event queue is full.
type OverflowPolicy string

const (
	OverflowDropOldest OverflowPolicy = "drop_oldest" // discard the oldest buffered event
	OverflowDropNewest OverflowPolicy = "drop_newest" // discard the incoming event
	OverflowBlock      OverflowPolicy = "block"       // apply backpressure to the caller
)

// RetryConfig controls retry behaviour for failed HTTP requests.
type RetryConfig struct {
	MaxRetries     int           // max retry attempts (default: 3)
	InitialBackoff time.Duration // first retry delay (default: 100ms)
	MaxBackoff     time.Duration // cap on exponential backoff (default: 30s)
	Multiplier     float64       // backoff multiplier (default: 2.0)
	Jitter         bool          // add random jitter to backoff (default: true)
	// HTTP status codes to retry (default: 429, 500, 502, 503, 504).
	RetryableErrors []int
}

// RateLimitConfig enforces a per-provider token-bucket rate limit.
type RateLimitConfig struct {
	RequestsPerSecond int // token bucket rate (default: provider-specific)
	BurstSize         int // max burst tokens (default: RequestsPerSecond * 2)
}

// ProviderConfig holds initialization settings common to all provider implementations.
type ProviderConfig struct {
	// Secret management
	APIKey     string
	SecretType SecretType // env_var | file | vault | inline

	// Proxy settings (bypass ad-blockers)
	ProxyURL  string    // e.g., https://analytics.yourcompany.com/amp
	ProxyMode ProxyMode // direct | reverse_proxy | custom

	// Batching & queue settings
	BatchSize      int            // max events per batch (default: provider-specific)
	FlushInterval  time.Duration  // auto-flush interval (default: 10s)
	MaxQueueSize   int            // buffered events limit (default: 10000)
	OverflowPolicy OverflowPolicy // drop_oldest | drop_newest | block

	// Retry & backoff
	RetryConfig RetryConfig

	// Rate limiting (per-provider)
	RateLimitConfig RateLimitConfig

	// HTTP transport overrides
	Timeout      time.Duration
	MaxIdleConns int
	TLSConfig    *tls.Config
}
