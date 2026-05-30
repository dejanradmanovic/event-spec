package io.eventspec.analytics.amplitude

import io.eventspec.analytics.*
import java.time.Instant
import kotlin.test.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.*

class AmplitudeProviderTest {
  private fun provider() = AmplitudeProvider(AmplitudeConfig(apiKey = "test-key"))

  // ── metadata ──────────────────────────────────────────────────────────────

  @Test
  fun `metadata returns amplitude name`() {
    assertEquals("amplitude", provider().metadata().name)
  }

  @Test
  fun `metadata reports group and alias as supported`() {
    val caps = provider().metadata().capabilities
    assertTrue(caps.track)
    assertTrue(caps.identify)
    assertTrue(caps.group)
    assertFalse(caps.page)
    assertTrue(caps.alias)
  }

  // ── unsupported operation ─────────────────────────────────────────────────

  @Test
  fun `page throws UnsupportedOperationException`() = runTest {
    assertFailsWith<UnsupportedOperationException> {
      provider().page(PageMessage("id", Instant.now(), "u", "a", "/home", emptyMap()))
    }
  }

  // ── TrackMessage mapper ───────────────────────────────────────────────────

  @Test
  fun `toAmplitudeEvent maps track fields correctly`() {
    val msg =
        TrackMessage(
            messageId = "m1",
            timestamp = Instant.ofEpochMilli(1000),
            eventName = "Product Viewed",
            properties = mapOf("product_id" to "sku-1"),
            userId = "user-1",
            anonymousId = "anon-1",
        )
    val ev = msg.toAmplitudeEvent()

    assertEquals("Product Viewed", ev.eventType)
    assertEquals("user-1", ev.userId)
    assertEquals("anon-1", ev.deviceId)
    assertEquals(1000L, ev.time)
    assertEquals("m1", ev.insertId)
    assertEquals(JsonPrimitive("sku-1"), ev.eventProperties?.get("product_id"))
    assertNull(ev.userProperties)
  }

  @Test
  fun `toAmplitudeEvent omits empty userId and anonymousId`() {
    val msg =
        TrackMessage(
            messageId = "m",
            timestamp = Instant.now(),
            eventName = "Test",
            properties = emptyMap(),
            userId = "",
            anonymousId = "",
        )
    val ev = msg.toAmplitudeEvent()
    assertNull(ev.userId)
    assertNull(ev.deviceId)
  }

  @Test
  fun `toAmplitudeEvent applies MessageContext ip and locale`() {
    val msg =
        TrackMessage(
            messageId = "m",
            timestamp = Instant.now(),
            eventName = "Test",
            properties = emptyMap(),
            userId = "u",
            anonymousId = "a",
            context = MessageContext(ipAddress = "1.2.3.4", locale = "en-US"),
        )
    val ev = msg.toAmplitudeEvent()
    assertEquals("1.2.3.4", ev.ip)
    assertEquals("en-US", ev.language)
  }

  @Test
  fun `toAmplitudeEvent merges extra context into eventProperties`() {
    val msg =
        TrackMessage(
            messageId = "m",
            timestamp = Instant.now(),
            eventName = "Test",
            properties = mapOf("existing" to "value"),
            userId = "u",
            anonymousId = "a",
            context = MessageContext(extra = mapOf("session_id" to "sess-1")),
        )
    val ev = msg.toAmplitudeEvent()
    assertEquals(JsonPrimitive("value"), ev.eventProperties?.get("existing"))
    assertEquals(JsonPrimitive("sess-1"), ev.eventProperties?.get("session_id"))
  }

  @Test
  fun `toAmplitudeEvent extra does not overwrite existing eventProperties`() {
    val msg =
        TrackMessage(
            messageId = "m",
            timestamp = Instant.now(),
            eventName = "Test",
            properties = mapOf("key" to "from-props"),
            userId = "u",
            anonymousId = "a",
            context = MessageContext(extra = mapOf("key" to "from-extra")),
        )
    val ev = msg.toAmplitudeEvent()
    assertEquals(JsonPrimitive("from-props"), ev.eventProperties?.get("key"))
  }

  // ── IdentifyMessage mapper ────────────────────────────────────────────────

  @Test
  fun `toAmplitudeEvent maps identify to $identify with $set traits`() {
    val msg =
        IdentifyMessage(
            messageId = "m2",
            timestamp = Instant.ofEpochMilli(2000),
            userId = "user-1",
            anonymousId = "anon-1",
            traits = mapOf("email" to "alice@example.com", "plan" to "pro"),
        )
    val ev = msg.toAmplitudeEvent()

    assertEquals("\$identify", ev.eventType)
    assertEquals("user-1", ev.userId)
    assertEquals("anon-1", ev.deviceId)
    assertEquals(2000L, ev.time)
    assertEquals("m2", ev.insertId)
    assertNull(ev.eventProperties)

    val set = ev.userProperties?.get("\$set")?.jsonObject
    assertNotNull(set)
    assertEquals(JsonPrimitive("alice@example.com"), set["email"])
    assertEquals(JsonPrimitive("pro"), set["plan"])
  }

  // ── GroupMessage mapper ───────────────────────────────────────────────────

  @Test
  fun `toAmplitudeEvent maps group to $groupidentify`() {
    val msg =
        GroupMessage(
            messageId = "m3",
            timestamp = Instant.ofEpochMilli(3000),
            userId = "user-1",
            anonymousId = "anon-1",
            groupId = "org-456",
            traits = mapOf("name" to "Acme Corp"),
        )
    val ev = msg.toAmplitudeEvent()

    assertEquals("\$groupidentify", ev.eventType)
    assertEquals("group", ev.groupType)
    assertEquals("org-456", ev.groupValue)
    assertEquals("m3", ev.insertId)

    val set = ev.groupProperties?.get("\$set")?.jsonObject
    assertNotNull(set)
    assertEquals(JsonPrimitive("Acme Corp"), set["name"])
  }

  // ── AliasMessage mapper ───────────────────────────────────────────────────

  @Test
  fun `toAmplitudeEvent maps alias to $identify with previousId as deviceId`() {
    val msg =
        AliasMessage(
            messageId = "m4",
            timestamp = Instant.ofEpochMilli(4000),
            userId = "user-1",
            previousId = "anon-old",
        )
    val ev = msg.toAmplitudeEvent()

    assertEquals("\$identify", ev.eventType)
    assertEquals("user-1", ev.userId)
    assertEquals("anon-old", ev.deviceId)
    assertEquals(4000L, ev.time)
    assertNull(ev.userProperties)
    assertNull(ev.eventProperties)
  }

  // ── Property coercion ─────────────────────────────────────────────────────

  @Test
  fun `truncateString keeps strings under limit unchanged`() {
    val s = "hello"
    assertEquals(s, truncateString(s))
  }

  @Test
  fun `truncateString truncates at MAX_STRING_CHARS code points`() {
    val long = "a".repeat(2000)
    val truncated = truncateString(long)
    assertEquals(1024, truncated.length)
  }

  @Test
  fun `coerceProperties handles nested maps and arrays`() {
    val props: Map<String, Any?> =
        mapOf(
            "nested" to mapOf("a" to 1),
            "list" to listOf("x", "y"),
            "num" to 3.14,
            "flag" to true,
            "nil" to null,
        )
    val out = coerceProperties(props)
    assertNotNull(out)
    assertEquals(JsonObject(mapOf("a" to JsonPrimitive(1))), out["nested"])
    assertEquals(JsonArray(listOf(JsonPrimitive("x"), JsonPrimitive("y"))), out["list"])
    assertEquals(JsonPrimitive(3.14), out["num"])
    assertEquals(JsonPrimitive(true), out["flag"])
    assertEquals(JsonNull, out["nil"])
  }

  // ── track enqueue ─────────────────────────────────────────────────────────

  @Test
  fun `track enqueues event without throwing`() = runTest {
    val p = provider()
    p.track(
        TrackMessage(
            messageId = "msg-1",
            timestamp = Instant.now(),
            eventName = "Button Clicked",
            properties = mapOf("label" to "signup"),
            userId = "user-1",
            anonymousId = "anon-1",
        )
    )
  }
}
