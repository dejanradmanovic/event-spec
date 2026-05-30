package io.eventspec.analytics.amplitude

import io.eventspec.analytics.*
import java.io.OutputStreamWriter
import java.net.HttpURLConnection
import java.net.URL
import kotlin.math.min
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
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
  private val pc = config.providerConfig
  private val rc = pc.retryConfig
  private val rl = pc.rateLimitConfig
  private val effectiveEndpoint = resolveEndpoint(config.endpoint, pc)
  private val rateLimitIntervalMs: Long =
      if (rl.requestsPerSecond > 0) 1000L / rl.requestsPerSecond else 0L
  private val rateMutex = Mutex()
  private var nextSendTime = 0L

  private val queue =
      EventQueue<TrackMessage>(
          onFlush = { batch -> sendBatch(batch.map { it.toAmplitudeEvent() }) },
          opts =
              QueueOptions(
                  batchSize = pc.batchSize,
                  flushIntervalMs = pc.flushIntervalMs,
                  maxSize = pc.maxQueueSize,
                  overflowPolicy = pc.overflowPolicy,
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

    // Rate limiting: reserve a send slot and delay until it opens.
    if (rateLimitIntervalMs > 0) {
      val waitUntil = rateMutex.withLock {
        val now = System.currentTimeMillis()
        val next = maxOf(now, nextSendTime)
        nextSendTime = next + rateLimitIntervalMs
        next
      }
      val now = System.currentTimeMillis()
      if (waitUntil > now) delay(waitUntil - now)
    }

    var lastError: Exception? = null
    var backoff = rc.initialBackoffMs
    for (attempt in 0..rc.maxRetries) {
      if (attempt > 0) {
        val delayMs = if (rc.jitter) (backoff * (0.5 + Math.random() * 0.5)).toLong() else backoff
        delay(delayMs)
        backoff = min((backoff.toDouble() * rc.multiplier).toLong(), rc.maxBackoffMs)
      }
      try {
        postJson(effectiveEndpoint, body)
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

// resolveEndpoint computes the effective HTTP endpoint, applying proxy rewriting from pc.
private fun resolveEndpoint(endpoint: String, pc: ProviderConfig): String {
  val proxyUrl = pc.proxyUrl ?: return endpoint
  return when (pc.proxyMode) {
    ProxyMode.REVERSE_PROXY -> {
      try {
        val proxy = URL(proxyUrl)
        val target = URL(endpoint)
        val basePath = proxy.path.trimEnd('/')
        val provPath = target.path.trimStart('/')
        val newPath = if (basePath.isEmpty()) "/$provPath" else "$basePath/$provPath"
        val port = if (proxy.port != -1) ":${proxy.port}" else ""
        "${proxy.protocol}://${proxy.host}${port}${newPath}"
      } catch (e: Exception) {
        endpoint
      }
    }
    ProxyMode.CUSTOM -> proxyUrl
    ProxyMode.DIRECT -> endpoint
  }
}
