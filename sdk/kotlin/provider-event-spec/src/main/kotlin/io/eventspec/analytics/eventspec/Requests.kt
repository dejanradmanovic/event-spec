package io.eventspec.analytics.eventspec

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class ContextPayload(
    @SerialName("user_id") val userId: String? = null,
    @SerialName("anonymous_id") val anonymousId: String? = null,
    @SerialName("attributes") val attributes: Map<String, String>? = null,
)

@Serializable
data class TrackRequest(
    @SerialName("source") val source: String,
    @SerialName("event_name") val eventName: String,
    @SerialName("properties") val properties: Map<String, String>? = null,
    @SerialName("context") val context: ContextPayload,
    @SerialName("timestamp") val timestamp: String,
)

@Serializable
data class IdentifyRequest(
    @SerialName("source") val source: String,
    @SerialName("user_id") val userId: String? = null,
    @SerialName("anonymous_id") val anonymousId: String? = null,
    @SerialName("traits") val traits: Map<String, String>? = null,
    @SerialName("timestamp") val timestamp: String,
)

@Serializable
data class GroupRequest(
    @SerialName("source") val source: String,
    @SerialName("user_id") val userId: String? = null,
    @SerialName("anonymous_id") val anonymousId: String? = null,
    @SerialName("group_id") val groupId: String,
    @SerialName("traits") val traits: Map<String, String>? = null,
    @SerialName("timestamp") val timestamp: String,
)

@Serializable
data class PageRequest(
    @SerialName("source") val source: String,
    @SerialName("user_id") val userId: String? = null,
    @SerialName("anonymous_id") val anonymousId: String? = null,
    @SerialName("name") val name: String,
    @SerialName("properties") val properties: Map<String, String>? = null,
    @SerialName("timestamp") val timestamp: String,
)

@Serializable
data class AliasRequest(
    @SerialName("source") val source: String,
    @SerialName("user_id") val userId: String? = null,
    @SerialName("previous_id") val previousId: String? = null,
    @SerialName("timestamp") val timestamp: String,
)

@Serializable
data class FlushRequest(
    @SerialName("source") val source: String,
)
