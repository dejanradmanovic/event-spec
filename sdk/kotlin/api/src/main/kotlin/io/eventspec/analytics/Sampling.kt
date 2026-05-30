package io.eventspec.analytics

enum class SamplingStrategy {
  USER_ID_HASH,
  RANDOM,
  NONE
}

data class SamplingPolicy(
    val strategy: SamplingStrategy,
    val rate: Double,
)

class SampledError : Exception("event dropped by sampling policy")

typealias SamplingLookup = (eventName: String) -> SamplingPolicy?

// SamplingHook applies the declared sampling policy during the Before stage.
// Sampled-out events throw SampledError; events without a policy pass through.
class SamplingHook(private val lookup: SamplingLookup) : UnimplementedHook() {
  override suspend fun before(hc: HookContext, hints: HookHints): EventEnvelope? {
    val policy = lookup(hc.eventName) ?: return null
    if (policy.strategy == SamplingStrategy.NONE) return null

    val keep =
        when (policy.strategy) {
          SamplingStrategy.USER_ID_HASH -> hashKeep(hc.context.userId, policy.rate)
          SamplingStrategy.RANDOM -> Math.random() < policy.rate
          SamplingStrategy.NONE -> true
        }
    if (!keep) throw SampledError()
    return null
  }
}

// fnv1a32 computes FNV-1a 32-bit hash, matching the Go and TypeScript implementations.
private fun fnv1a32(s: String): Long {
  val bytes = s.toByteArray(Charsets.UTF_8)
  var h = 2166136261L
  for (b in bytes) {
    h = h xor (b.toLong() and 0xFF)
    h = (h * 16777619L) and 0xFFFFFFFFL
  }
  return h
}

private fun hashKeep(userId: String, rate: Double): Boolean {
  val norm = fnv1a32(userId).toDouble() / 0x100000000L.toDouble()
  return norm < rate
}
