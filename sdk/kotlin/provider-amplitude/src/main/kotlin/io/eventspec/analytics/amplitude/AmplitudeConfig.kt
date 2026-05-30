package io.eventspec.analytics.amplitude

import io.eventspec.analytics.OverflowPolicy

const val DEFAULT_ENDPOINT = "https://api2.amplitude.com/batch"

enum class ProxyMode {
  DIRECT,
  REVERSE_PROXY,
  CUSTOM,
}

data class AmplitudeConfig(
    val apiKey: String,
    val endpoint: String = DEFAULT_ENDPOINT,

    // Proxy settings — override the destination URL to bypass ad-blockers.
    // REVERSE_PROXY: replace scheme+host with proxyUrl, keep the provider path.
    // CUSTOM: use proxyUrl as the complete replacement URL.
    val proxyUrl: String? = null,
    val proxyMode: ProxyMode = ProxyMode.DIRECT,

    // Batching & queue
    val batchSize: Int = 100,
    val flushIntervalMs: Long = 10_000,
    val maxQueueSize: Int = 10_000,
    val overflowPolicy: OverflowPolicy = OverflowPolicy.DROP_OLDEST,

    // Retry & backoff
    val maxRetries: Int = 3,
    val initialBackoffMs: Long = 100L,
    val maxBackoffMs: Long = 30_000L,
    val retryMultiplier: Double = 2.0,
    val jitter: Boolean = true,

    // Rate limiting
    val requestsPerSecond: Int = 0,
)
