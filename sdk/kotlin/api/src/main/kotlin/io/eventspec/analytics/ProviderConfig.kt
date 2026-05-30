package io.eventspec.analytics

// ProxyMode controls how HTTP traffic is routed to the provider's API.
enum class ProxyMode {
  DIRECT, // no proxy (default)
  REVERSE_PROXY, // replace scheme+host with proxyUrl, keep provider path
  CUSTOM, // use proxyUrl as the complete replacement URL
}

// RetryConfig controls retry behaviour for failed HTTP requests.
data class RetryConfig(
    val maxRetries: Int = 3,
    val initialBackoffMs: Long = 100L,
    val maxBackoffMs: Long = 30_000L,
    val multiplier: Double = 2.0,
    val jitter: Boolean = true,
)

// RateLimitConfig enforces a per-provider request rate limit.
data class RateLimitConfig(
    val requestsPerSecond: Int = 0, // 0 = unlimited
)

// ProviderConfig holds construction settings common to all provider implementations.
// Compose this into provider-specific config data classes.
data class ProviderConfig(
    // Proxy — override the destination URL to bypass ad-blockers.
    val proxyUrl: String? = null,
    val proxyMode: ProxyMode = ProxyMode.DIRECT,

    // Batching & queue
    val batchSize: Int = 100,
    val flushIntervalMs: Long = 10_000,
    val maxQueueSize: Int = 10_000,
    val overflowPolicy: OverflowPolicy = OverflowPolicy.DROP_OLDEST,

    // Retry & backoff
    val retryConfig: RetryConfig = RetryConfig(),

    // Rate limiting
    val rateLimitConfig: RateLimitConfig = RateLimitConfig(),
)
