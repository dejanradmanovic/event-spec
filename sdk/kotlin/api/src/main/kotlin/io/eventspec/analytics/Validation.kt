package io.eventspec.analytics

class ValidationError(message: String) : Exception(message)

typealias SchemaValidator = (eventName: String, properties: Map<String, Any?>) -> String?

// ValidationHook validates event properties against a schema before dispatch.
// The validator returns a non-null error message on failure, or null on success.
class ValidationHook(private val validator: SchemaValidator) : UnimplementedHook() {
  override suspend fun before(hc: HookContext, hints: HookHints): EventEnvelope? {
    val properties =
        (hc.message as? Map<*, *>)?.let {
          @Suppress("UNCHECKED_CAST")
          it as? Map<String, Any?>
        } ?: emptyMap()
    val err = validator(hc.eventName, properties)
    if (err != null) throw ValidationError(err)
    return null
  }
}
