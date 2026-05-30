package io.eventspec.analytics.eventspec

import io.eventspec.analytics.ProviderConfig

// EventSpecConfig composes the shared ProviderConfig with event-spec server fields.
data class EventSpecConfig(
    val baseUrl: String,
    val apiKey: String,
    val source: String,
    val providerConfig: ProviderConfig = ProviderConfig(),
)
