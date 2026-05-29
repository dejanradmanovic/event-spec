package io.eventspec.analytics.amplitude

const val DEFAULT_ENDPOINT = "https://api2.amplitude.com/batch"

data class AmplitudeConfig(
    val apiKey: String,
    val endpoint: String = DEFAULT_ENDPOINT,
    val batchSize: Int = 100,
    val flushIntervalMs: Long = 10_000,
    val maxRetries: Int = 3,
)
