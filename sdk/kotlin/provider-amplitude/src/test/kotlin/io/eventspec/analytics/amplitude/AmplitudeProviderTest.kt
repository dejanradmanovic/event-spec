package io.eventspec.analytics.amplitude

import io.eventspec.analytics.*
import java.time.Instant
import kotlin.test.*
import kotlinx.coroutines.test.runTest

class AmplitudeProviderTest {
  private fun provider() = AmplitudeProvider(AmplitudeConfig(apiKey = "test-key"))

  @Test
  fun `metadata returns amplitude name`() {
    assertEquals("amplitude", provider().metadata().name)
  }

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
        ))
  }

  @Test
  fun `group throws UnsupportedOperationException`() = runTest {
    assertFailsWith<UnsupportedOperationException> {
      provider().group(GroupMessage("id", Instant.now(), "u", "a", "g", emptyMap()))
    }
  }

  @Test
  fun `page throws UnsupportedOperationException`() = runTest {
    assertFailsWith<UnsupportedOperationException> {
      provider().page(PageMessage("id", Instant.now(), "u", "a", "/home", emptyMap()))
    }
  }

  @Test
  fun `toAmplitudeEvent maps fields correctly`() {
    val msg =
        TrackMessage(
            messageId = "m",
            timestamp = Instant.ofEpochMilli(1000),
            eventName = "Product Viewed",
            properties = mapOf("product_id" to "sku-1"),
            userId = "user-1",
            anonymousId = "anon-1",
        )
    val event = msg.toAmplitudeEvent()
    assertEquals("Product Viewed", event.eventType)
    assertEquals("user-1", event.userId)
    assertEquals("anon-1", event.deviceId)
    assertEquals(1000L, event.time)
    assertEquals("sku-1", event.eventProperties["product_id"])
  }
}
