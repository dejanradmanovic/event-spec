package io.eventspec.analytics.eventspec

import io.eventspec.analytics.*
import java.io.OutputStreamWriter
import java.net.HttpURLConnection
import java.net.URL
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.math.min
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

// EventSpecProvider is a thin-client Provider that forwards every analytics call
// to an event-spec runtime ingestion server over HTTP.
// Capabilities: Track ✅, Identify ✅, Group ✅, Page ✅, Alias ✅.
class EventSpecProvider(private val config: EventSpecConfig) : Provider {

  private val json = Json { encodeDefaults = false }
  private val pc = config.providerConfig
  private val rc = pc.retryConfig
  private val rl = pc.rateLimitConfig
  private val effectiveBaseUrl = resolveEndpoint(config.baseUrl, pc)
  private val rateLimitIntervalMs: Long =
      if (rl.requestsPerSecond > 0) 1000L / rl.requestsPerSecond else 0L
  private val rateMutex = Mutex()
  private var nextSendTime = 0L
  private val closed = AtomicBoolean(false)

  override fun metadata() =
      ProviderMetadata(
          name = "event-spec",
          version = "1.0.0",
          capabilities =
              ProviderCapabilities(
                  track = true,
                  identify = true,
                  group = true,
                  page = true,
                  alias = true,
              ),
      )

  override fun hooks(): List<Hook> = emptyList()

  override suspend fun track(msg: TrackMessage) {
    checkOpen()
    post(
        "/v1/track",
        json.encodeToString(
            TrackRequest(
                source = config.source,
                eventName = msg.eventName,
                properties = msg.properties.mapValues { (_, v) -> v?.toString() ?: "" },
                context =
                    ContextPayload(
                        userId = msg.userId.ifEmpty { null },
                        anonymousId = msg.anonymousId.ifEmpty { null },
                        attributes = buildAttributes(msg.context),
                    ),
                timestamp = msg.timestamp.toString(),
            )
        ),
    )
  }

  override suspend fun identify(msg: IdentifyMessage) {
    checkOpen()
    post(
        "/v1/identify",
        json.encodeToString(
            IdentifyRequest(
                source = config.source,
                userId = msg.userId.ifEmpty { null },
                anonymousId = msg.anonymousId.ifEmpty { null },
                traits = msg.traits.mapValues { (_, v) -> v?.toString() ?: "" },
                timestamp = msg.timestamp.toString(),
            )
        ),
    )
  }

  override suspend fun group(msg: GroupMessage) {
    checkOpen()
    post(
        "/v1/group",
        json.encodeToString(
            GroupRequest(
                source = config.source,
                userId = msg.userId.ifEmpty { null },
                anonymousId = msg.anonymousId.ifEmpty { null },
                groupId = msg.groupId,
                traits = msg.traits.mapValues { (_, v) -> v?.toString() ?: "" },
                timestamp = msg.timestamp.toString(),
            )
        ),
    )
  }

  override suspend fun page(msg: PageMessage) {
    checkOpen()
    post(
        "/v1/page",
        json.encodeToString(
            PageRequest(
                source = config.source,
                userId = msg.userId.ifEmpty { null },
                anonymousId = msg.anonymousId.ifEmpty { null },
                name = msg.name,
                properties = msg.properties.mapValues { (_, v) -> v?.toString() ?: "" },
                timestamp = msg.timestamp.toString(),
            )
        ),
    )
  }

  override suspend fun alias(msg: AliasMessage) {
    checkOpen()
    post(
        "/v1/alias",
        json.encodeToString(
            AliasRequest(
                source = config.source,
                userId = msg.userId.ifEmpty { null },
                previousId = msg.previousId.ifEmpty { null },
                timestamp = msg.timestamp.toString(),
            )
        ),
    )
  }

  override suspend fun flush() {
    checkOpen()
    post("/v1/flush", json.encodeToString(FlushRequest(source = config.source)))
  }

  override suspend fun shutdown() {
    if (!closed.compareAndSet(false, true)) {
      throw IllegalStateException("event-spec provider: already shut down")
    }
    post("/v1/flush", json.encodeToString(FlushRequest(source = config.source)))
  }

  private fun checkOpen() {
    if (closed.get()) throw IllegalStateException("event-spec provider: already shut down")
  }

  // post is the internal dispatch — does not check the closed state. Callers are responsible.
  private suspend fun post(path: String, body: String) {
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
        postJson(effectiveBaseUrl + path, body)
        return
      } catch (e: Exception) {
        lastError = e
      }
    }
    throw lastError ?: Exception("event-spec provider: request to $path failed")
  }

  private suspend fun postJson(url: String, body: String) =
      withContext(Dispatchers.IO) {
        val conn = URL(url).openConnection() as HttpURLConnection
        try {
          conn.requestMethod = "POST"
          conn.setRequestProperty("Content-Type", "application/json")
          conn.setRequestProperty("Accept", "application/json")
          conn.setRequestProperty("Authorization", "Bearer ${config.apiKey}")
          conn.doOutput = true
          conn.connectTimeout = 10_000
          conn.readTimeout = 30_000

          OutputStreamWriter(conn.outputStream, Charsets.UTF_8).use { it.write(body) }

          val status = conn.responseCode
          if (status < 200 || status >= 300) {
            throw Exception("event-spec provider: unexpected status $status from $url")
          }
        } finally {
          conn.disconnect()
        }
      }

  // buildAttributes merges standard MessageContext fields into a flat map for the server.
  private fun buildAttributes(mc: MessageContext): Map<String, String>? {
    val attrs = mutableMapOf<String, String>()
    mc.userAgent?.takeIf { it.isNotEmpty() }?.let { attrs["user_agent"] = it }
    mc.ipAddress?.takeIf { it.isNotEmpty() }?.let { attrs["ip_address"] = it }
    mc.extra?.forEach { (k, v) -> attrs[k] = v.toString() }
    return if (attrs.isEmpty()) null else attrs
  }
}

// resolveEndpoint computes the effective base URL, applying proxy rewriting from pc.
private fun resolveEndpoint(baseUrl: String, pc: ProviderConfig): String {
  val proxyUrl = pc.proxyUrl ?: return baseUrl
  return when (pc.proxyMode) {
    ProxyMode.REVERSE_PROXY -> {
      try {
        val proxy = URL(proxyUrl)
        val target = URL(baseUrl)
        val basePath = proxy.path.trimEnd('/')
        val provPath = target.path.trimStart('/')
        val newPath =
            when {
              basePath.isEmpty() && provPath.isEmpty() -> ""
              basePath.isEmpty() -> "/$provPath"
              provPath.isEmpty() -> basePath
              else -> "$basePath/$provPath"
            }
        val port = if (proxy.port != -1) ":${proxy.port}" else ""
        "${proxy.protocol}://${proxy.host}${port}${newPath}"
      } catch (e: Exception) {
        baseUrl
      }
    }
    ProxyMode.CUSTOM -> proxyUrl
    ProxyMode.DIRECT -> baseUrl
  }
}
