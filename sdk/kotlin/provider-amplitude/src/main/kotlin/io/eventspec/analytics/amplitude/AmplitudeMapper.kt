package io.eventspec.analytics.amplitude

import io.eventspec.analytics.*
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.*

private const val MAX_STRING_CHARS = 1024

// AmplitudeEvent is the Amplitude HTTP batch event wire format.
// All fields use snake_case to match the Amplitude API schema exactly.
@Serializable
data class AmplitudeEvent(
    @SerialName("event_type") val eventType: String,
    @SerialName("time") val time: Long,
    @SerialName("user_id") val userId: String? = null,
    @SerialName("device_id") val deviceId: String? = null,
    @SerialName("event_properties") val eventProperties: JsonObject? = null,
    @SerialName("user_properties") val userProperties: JsonObject? = null,
    @SerialName("group_type") val groupType: String? = null,
    @SerialName("group_value") val groupValue: String? = null,
    @SerialName("group_properties") val groupProperties: JsonObject? = null,
    @SerialName("insert_id") val insertId: String? = null,
    @SerialName("ip") val ip: String? = null,
    @SerialName("language") val language: String? = null,
    @SerialName("os_name") val osName: String? = null,
    @SerialName("platform") val platform: String? = null,
)

@Serializable
data class AmplitudeBatchRequest(
    @SerialName("api_key") val apiKey: String,
    @SerialName("events") val events: List<AmplitudeEvent>,
)

fun TrackMessage.toAmplitudeEvent(): AmplitudeEvent {
  val ev =
      AmplitudeEvent(
          eventType = eventName,
          time = timestamp.toEpochMilli(),
          userId = userId.ifEmpty { null },
          deviceId = anonymousId.ifEmpty { null },
          eventProperties = coerceProperties(properties),
          insertId = messageId.ifEmpty { null },
      )
  return applyMessageContext(ev, context)
}

fun IdentifyMessage.toAmplitudeEvent(): AmplitudeEvent {
  val ev =
      AmplitudeEvent(
          eventType = "\$identify",
          time = timestamp.toEpochMilli(),
          userId = userId.ifEmpty { null },
          deviceId = anonymousId.ifEmpty { null },
          userProperties =
              buildJsonObject { put("\$set", coerceProperties(traits) ?: JsonObject(emptyMap())) },
          insertId = messageId.ifEmpty { null },
      )
  return applyMessageContext(ev, context)
}

fun GroupMessage.toAmplitudeEvent(): AmplitudeEvent {
  val ev =
      AmplitudeEvent(
          eventType = "\$groupidentify",
          time = timestamp.toEpochMilli(),
          userId = userId.ifEmpty { null },
          deviceId = anonymousId.ifEmpty { null },
          groupType = "group",
          groupValue = groupId,
          groupProperties =
              buildJsonObject { put("\$set", coerceProperties(traits) ?: JsonObject(emptyMap())) },
          insertId = messageId.ifEmpty { null },
      )
  return applyMessageContext(ev, context)
}

// AliasMessage maps to a $identify event that links previousId (device_id) to
// userId, merging the two identities in Amplitude.
fun AliasMessage.toAmplitudeEvent(): AmplitudeEvent {
  val ev =
      AmplitudeEvent(
          eventType = "\$identify",
          time = timestamp.toEpochMilli(),
          userId = userId.ifEmpty { null },
          deviceId = previousId.ifEmpty { null },
          insertId = messageId.ifEmpty { null },
      )
  return applyMessageContext(ev, context)
}

// applyMessageContext maps standard MessageContext fields to their Amplitude
// equivalents and merges Extra attributes into eventProperties.
private fun applyMessageContext(ev: AmplitudeEvent, mc: MessageContext): AmplitudeEvent {
  val ip = mc.ipAddress?.takeIf { it.isNotEmpty() }
  val language = mc.locale?.takeIf { it.isNotEmpty() }
  val osName = (mc.os?.get("name") as? String)?.takeIf { it.isNotEmpty() }
  val platform = (mc.app?.get("platform") as? String)?.takeIf { it.isNotEmpty() }

  val extra: Map<String, Any>? = mc.extra
  val mergedEventProps =
      if (!extra.isNullOrEmpty()) {
        val existing = ev.eventProperties?.toMutableMap() ?: mutableMapOf()
        for ((k, v) in extra) {
          if (k !in existing) {
            existing[k] = coerceValue(v as Any?)
          }
        }
        JsonObject(existing)
      } else {
        ev.eventProperties
      }

  return ev.copy(
      ip = ip,
      language = language,
      osName = osName,
      platform = platform,
      eventProperties = mergedEventProps,
  )
}

// coerceProperties recursively applies Amplitude property constraints.
internal fun coerceProperties(props: Map<String, Any?>?): JsonObject? {
  if (props == null) return null
  return JsonObject(props.mapValues { (_, v) -> coerceValue(v) })
}

private fun coerceValue(v: Any?): JsonElement =
    when (v) {
      null -> JsonNull
      is String -> JsonPrimitive(truncateString(v))
      is Number -> JsonPrimitive(v)
      is Boolean -> JsonPrimitive(v)
      is Map<*, *> -> {
        @Suppress("UNCHECKED_CAST")
        coerceProperties(v as Map<String, Any?>) ?: JsonObject(emptyMap())
      }
      is List<*> -> JsonArray(v.map { coerceValue(it) })
      is Array<*> -> JsonArray(v.map { coerceValue(it) })
      else -> JsonPrimitive(v.toString())
    }

// truncateString truncates s to at most MAX_STRING_CHARS Unicode code points.
internal fun truncateString(s: String): String {
  val codePoints = s.codePoints().toArray()
  if (codePoints.size <= MAX_STRING_CHARS) return s
  return String(codePoints, 0, MAX_STRING_CHARS)
}
