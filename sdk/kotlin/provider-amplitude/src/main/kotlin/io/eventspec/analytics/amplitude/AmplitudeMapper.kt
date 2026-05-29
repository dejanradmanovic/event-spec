package io.eventspec.analytics.amplitude

import io.eventspec.analytics.IdentifyMessage
import io.eventspec.analytics.TrackMessage

// AmplitudeEvent is the wire format for the Amplitude HTTP API.
data class AmplitudeEvent(
    val eventType: String,
    val userId: String,
    val deviceId: String,
    val eventProperties: Map<String, Any?>,
    val time: Long,
)

data class AmplitudeIdentifyEvent(
    val userId: String,
    val deviceId: String,
    val userProperties: Map<String, Any?>,
    val time: Long,
)

fun TrackMessage.toAmplitudeEvent() =
    AmplitudeEvent(
        eventType = eventName,
        userId = userId,
        deviceId = anonymousId,
        eventProperties = properties,
        time = timestamp.toEpochMilli(),
    )

fun IdentifyMessage.toAmplitudeIdentifyEvent() =
    AmplitudeIdentifyEvent(
        userId = userId,
        deviceId = anonymousId,
        userProperties = traits,
        time = timestamp.toEpochMilli(),
    )
