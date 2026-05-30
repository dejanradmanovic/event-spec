package io.eventspec.analytics

data class AnalyticsContext(
    val userId: String = "",
    val anonymousId: String = "",
    val attributes: Map<String, Any?> = emptyMap(),
)

// merge combines base and override. Non-empty override fields win.
// Attributes are merged key-by-key with override keys winning.
fun merge(base: AnalyticsContext, override: AnalyticsContext): AnalyticsContext {
  val userId = override.userId.ifEmpty { base.userId }
  val anonymousId = override.anonymousId.ifEmpty { base.anonymousId }
  val attributes = base.attributes + override.attributes
  return AnalyticsContext(userId, anonymousId, attributes)
}
