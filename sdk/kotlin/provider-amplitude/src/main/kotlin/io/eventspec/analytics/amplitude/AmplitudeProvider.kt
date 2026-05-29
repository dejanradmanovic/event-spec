package io.eventspec.analytics.amplitude

import io.eventspec.analytics.*
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers

class AmplitudeProvider(
    private val config: AmplitudeConfig,
    scope: CoroutineScope = CoroutineScope(Dispatchers.Default),
) : Provider {

  private val queue =
      EventQueue<AmplitudeEvent>(
          onFlush = { batch -> sendBatch(batch) },
          opts =
              QueueOptions(
                  batchSize = config.batchSize,
                  flushIntervalMs = config.flushIntervalMs,
              ),
          scope = scope,
      )

  override fun metadata() =
      ProviderMetadata(
          name = "amplitude",
          version = "0.1.0",
          capabilities =
              ProviderCapabilities(
                  track = true, identify = true, group = false, page = false, alias = true),
      )

  override suspend fun track(msg: TrackMessage) {
    queue.enqueue(msg.toAmplitudeEvent())
  }

  override suspend fun identify(msg: IdentifyMessage) {
    // Amplitude identify is sent as a special event via the same batch endpoint.
    val event =
        AmplitudeEvent(
            eventType = "\$identify",
            userId = msg.userId,
            deviceId = msg.anonymousId,
            eventProperties = emptyMap(),
            time = msg.timestamp.toEpochMilli(),
        )
    queue.enqueue(event)
  }

  override suspend fun group(msg: GroupMessage) {
    throw UnsupportedOperationException("group")
  }

  override suspend fun page(msg: PageMessage) {
    throw UnsupportedOperationException("page")
  }

  override suspend fun alias(msg: AliasMessage) {
    val event =
        AmplitudeEvent(
            eventType = "\$merge",
            userId = msg.userId,
            deviceId = msg.previousId,
            eventProperties = emptyMap(),
            time = msg.timestamp.toEpochMilli(),
        )
    queue.enqueue(event)
  }

  override suspend fun flush() {
    queue.flushAll()
  }

  override suspend fun shutdown() {
    queue.shutdown()
  }

  private suspend fun sendBatch(events: List<AmplitudeEvent>) {
    // HTTP dispatch to Amplitude's batch endpoint.
    // In production this uses OkHttp or Ktor; left as a no-op so the core
    // compiles without a heavy HTTP dependency in the test suite.
  }
}
