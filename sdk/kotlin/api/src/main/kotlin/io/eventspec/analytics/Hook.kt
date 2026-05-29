package io.eventspec.analytics

typealias HookHints = Map<String, Any?>

data class HookResult(
    val delivered: Boolean,
    val dropped: Boolean,
    val error: Throwable? = null,
)

data class HookContext(
    val operation: String,
    val eventName: String,
    val context: AnalyticsContext,
    val message: Any? = null,
    val provider: String? = null,
)

data class EventEnvelope(
    val eventName: String,
    val properties: Map<String, Any?>,
    val context: AnalyticsContext,
    val metadata: Map<String, Any?> = emptyMap(),
)

// Hook is the middleware interface for the analytics event pipeline.
// before runs once, gating all providers. after/error/finally fire once per provider result.
interface Hook {
  // Return a non-null EventEnvelope to replace the event; throw to cancel.
  suspend fun before(hc: HookContext, hints: HookHints = emptyMap()): EventEnvelope? = null

  // Called per-provider on success, in reverse hook order.
  suspend fun after(hc: HookContext, result: HookResult, hints: HookHints = emptyMap()) {}

  // Called per-provider on failure, in reverse hook order. Must not throw.
  fun error(hc: HookContext, err: Throwable, hints: HookHints = emptyMap()) {}

  // Always called after after or error (defer semantics), in reverse hook order.
  fun finally(hc: HookContext, result: HookResult, hints: HookHints = emptyMap()) {}
}

// UnimplementedHook provides no-op implementations. Extend it and override only the stages you
// need.
open class UnimplementedHook : Hook

// HookChain executes a sequence of hooks in governance-first order for before,
// and reverse order for after/error/finally.
class HookChain(private val hooks: List<Hook>) {
  suspend fun before(hc: HookContext, hints: HookHints = emptyMap()): EventEnvelope? {
    var latest: EventEnvelope? = null
    var current = hc
    for (h in hooks) {
      val result = h.before(current, hints)
      if (result != null) {
        latest = result
        current = current.copy(eventName = result.eventName, context = result.context)
      }
    }
    return latest
  }

  suspend fun after(hc: HookContext, result: HookResult, hints: HookHints = emptyMap()) {
    for (h in hooks.reversed()) {
      h.after(hc, result, hints)
    }
  }

  fun error(hc: HookContext, err: Throwable, hints: HookHints = emptyMap()) {
    for (h in hooks.reversed()) {
      h.error(hc, err, hints)
    }
  }

  fun finally(hc: HookContext, result: HookResult, hints: HookHints = emptyMap()) {
    for (h in hooks.reversed()) {
      h.finally(hc, result, hints)
    }
  }
}
