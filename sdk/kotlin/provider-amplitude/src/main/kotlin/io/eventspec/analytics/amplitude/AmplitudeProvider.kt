package io.eventspec.analytics.amplitude

import io.eventspec.analytics.*
import java.io.OutputStreamWriter
import java.net.HttpURLConnection
import java.net.URL
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

// AmplitudeProvider sends analytics events to Amplitude via the HTTP batch API.
// Capabilities: Track ✅, Identify ✅, Group ✅, Page ❌ (unsupported), Alias ✅.
class AmplitudeProvider(
    private val config: AmplitudeConfig,
    scope: CoroutineScope = CoroutineScope(Dispatchers.Default),
) : Provider {

  private val json = Json { encodeDefaults = false }

  private val queue =
      EventQueue<TrackMessage>(
          onFlush = { batch -> sendBatch(batch.map { it.toAmplitudeEvent() }) },
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
                  track = true,
                  identify = true,
                  group = true,
                  page = false,
                  alias = true,
              ),
      )

  // track enqueues a track event for batched delivery.
  override suspend fun track(msg: TrackMessage) {
    queue.enqueue(msg)
  }

  // identify sends a $identify event synchronously so user-profile state arrives
  // at Amplitude before any subsequent track events that depend on it.
  override suspend fun identify(msg: IdentifyMessage) {
    sendBatch(listOf(msg.toAmplitudeEvent()))
  }

  // group sends a $groupidentify event synchronously for the same reason as identify.
  override suspend fun group(msg: GroupMessage) {
    sendBatch(listOf(msg.toAmplitudeEvent()))
  }

  // page is unsupported — Amplitude has no native page concept.
  override suspend fun page(msg: PageMessage) {
    throw UnsupportedOperationException("page")
  }

  // alias sends a $identify event synchronously, linking previousId to userId.
  override suspend fun alias(msg: AliasMessage) {
    sendBatch(listOf(msg.toAmplitudeEvent()))
  }

  override suspend fun flush() {
    queue.flushAll()
  }

  override suspend fun shutdown() {
    queue.shutdown()
  }

  // sendBatch serializes events and POSTs them to the Amplitude batch endpoint,
  // retrying with exponential backoff on transient failures.
  internal suspend fun sendBatch(events: List<AmplitudeEvent>) {
    if (events.isEmpty()) return
    val body = json.encodeToString(AmplitudeBatchRequest(apiKey = config.apiKey, events = events))

    var lastError: Exception? = null
    for (attempt in 0..config.maxRetries) {
      if (attempt > 0) {
        delay((1L shl (attempt - 1)) * 1000L) // 1s, 2s, 4s …
      }
      try {
        postJson(config.endpoint, body)
        return
      } catch (e: Exception) {
        lastError = e
      }
    }
    throw lastError ?: Exception("amplitude: send failed")
  }

  private suspend fun postJson(endpoint: String, body: String) =
      withContext(Dispatchers.IO) {
        val conn = URL(endpoint).openConnection() as HttpURLConnection
        try {
          conn.requestMethod = "POST"
          conn.setRequestProperty("Content-Type", "application/json")
          conn.setRequestProperty("Accept", "application/json")
          conn.doOutput = true
          conn.connectTimeout = 10_000
          conn.readTimeout = 30_000

          OutputStreamWriter(conn.outputStream, Charsets.UTF_8).use { it.write(body) }

          val status = conn.responseCode
          if (status != HttpURLConnection.HTTP_OK) {
            throw Exception("amplitude: unexpected status $status")
          }
        } finally {
          conn.disconnect()
        }
      }
}
