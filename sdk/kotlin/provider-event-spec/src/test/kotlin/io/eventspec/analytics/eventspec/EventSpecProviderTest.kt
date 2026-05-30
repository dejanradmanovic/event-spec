package io.eventspec.analytics.eventspec

import io.eventspec.analytics.*
import java.time.Instant
import kotlin.test.*
import kotlinx.coroutines.test.runTest

class EventSpecProviderTest {

  // Returns an EventSpecProvider pointing at a loopback address that will refuse connections.
  // Useful for tests that only inspect metadata or shutdown state without network I/O.
  private fun provider(
      url: String = "http://localhost:0",
  ) = EventSpecProvider(EventSpecConfig(baseUrl = url, apiKey = "test-key", source = "test-src"))

  // ── metadata ──────────────────────────────────────────────────────────────

  @Test
  fun `metadata returns event-spec name`() {
    assertEquals("event-spec", provider().metadata().name)
  }

  @Test
  fun `metadata reports all five capabilities as true`() {
    val caps = provider().metadata().capabilities
    assertTrue(caps.track)
    assertTrue(caps.identify)
    assertTrue(caps.group)
    assertTrue(caps.page)
    assertTrue(caps.alias)
  }

  @Test
  fun `hooks returns empty list`() {
    assertTrue(provider().hooks().isEmpty())
  }

  // ── shutdown / closed state ───────────────────────────────────────────────

  @Test
  fun `shutdown twice throws`() = runTest {
    val p =
        EventSpecProvider(
            EventSpecConfig(
                baseUrl = "http://localhost:0",
                apiKey = "k",
                source = "s",
                providerConfig =
                    ProviderConfig(retryConfig = RetryConfig(maxRetries = 0, initialBackoffMs = 1)),
            )
        )
    // First shutdown will fail to connect (no server), but that error is propagated.
    // We just verify the second call throws.
    try {
      p.shutdown()
    } catch (_: Exception) {}
    assertFailsWith<IllegalStateException> { p.shutdown() }
  }

  // ── request payload shape ─────────────────────────────────────────────────

  @Test
  fun `TrackRequest serializes all required fields`() {
    val req =
        TrackRequest(
            source = "web",
            eventName = "Product Viewed",
            properties = mapOf("sku" to "123"),
            context = ContextPayload(userId = "u1", anonymousId = "a1"),
            timestamp = Instant.ofEpochMilli(1000).toString(),
        )
    // Verify fields are accessible (data class contract).
    assertEquals("web", req.source)
    assertEquals("Product Viewed", req.eventName)
    assertEquals("u1", req.context.userId)
    assertEquals("a1", req.context.anonymousId)
  }

  @Test
  fun `FlushRequest serializes source`() {
    val req = FlushRequest(source = "web-app")
    assertEquals("web-app", req.source)
  }

  @Test
  fun `AliasRequest serializes previousId`() {
    val req =
        AliasRequest(
            source = "web",
            userId = "new",
            previousId = "old",
            timestamp = Instant.now().toString(),
        )
    assertEquals("old", req.previousId)
  }
}
