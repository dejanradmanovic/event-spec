package io.eventspec.analytics.amplitude

import io.eventspec.analytics.ProviderConfig

const val DEFAULT_ENDPOINT = "https://api2.amplitude.com/batch"

// AmplitudeConfig composes the shared ProviderConfig with Amplitude-specific fields.
data class AmplitudeConfig(
    val apiKey: String,
    val endpoint: String = DEFAULT_ENDPOINT,
    val providerConfig: ProviderConfig = ProviderConfig(),
)
