package io.eventspec.analytics

import kotlin.test.*
import kotlinx.coroutines.test.runTest

open class CaptureProvider(private val name: String = "capture") : Provider {
  val tracked = mutableListOf<TrackMessage>()
  val identified = mutableListOf<IdentifyMessage>()
  val grouped = mutableListOf<GroupMessage>()
  val paged = mutableListOf<PageMessage>()
  val aliased = mutableListOf<AliasMessage>()

  override fun metadata() = ProviderMetadata(name, "0.1.0")

  override fun hooks(): List<Hook> = emptyList()

  override suspend fun track(msg: TrackMessage) {
    tracked.add(msg)
  }

  override suspend fun identify(msg: IdentifyMessage) {
    identified.add(msg)
  }

  override suspend fun group(msg: GroupMessage) {
    grouped.add(msg)
  }

  override suspend fun page(msg: PageMessage) {
    paged.add(msg)
  }

  override suspend fun alias(msg: AliasMessage) {
    aliased.add(msg)
  }

  override suspend fun flush() {}

  override suspend fun shutdown() {}
}

class FailingProvider : CaptureProvider("failing") {
  override suspend fun track(msg: TrackMessage) {
    throw Exception("provider down")
  }
}

class ClientTest {
  @BeforeTest
  fun setUp() {
    setGlobalContext(AnalyticsContext())
    setGlobalProvider()
  }

  @Test
  fun `track dispatches to provider`() = runTest {
    val p = CaptureProvider()
    val client = Client(ClientOptions(providers = listOf(p)))
    client.track(Event("Button Clicked", mapOf("label" to "signup")))

    assertEquals(1, p.tracked.size)
    assertEquals("Button Clicked", p.tracked[0].eventName)
    assertEquals("signup", p.tracked[0].properties["label"])
  }

  @Test
  fun `track dispatches to multiple providers`() = runTest {
    val p1 = CaptureProvider("p1")
    val p2 = CaptureProvider("p2")
    val client = Client(ClientOptions(providers = listOf(p1, p2)))
    client.track(Event("Multi"))

    assertEquals(1, p1.tracked.size)
    assertEquals(1, p2.tracked.size)
  }

  @Test
  fun `track generates unique messageId per call`() = runTest {
    val p = CaptureProvider()
    val client = Client(ClientOptions(providers = listOf(p)))
    client.track(Event("E1"))
    client.track(Event("E2"))

    assertNotEquals(p.tracked[0].messageId, p.tracked[1].messageId)
  }

  @Test
  fun `trackDetailed returns success result`() = runTest {
    val p = CaptureProvider()
    val client = Client(ClientOptions(providers = listOf(p)))
    val result = client.trackDetailed(Event("Ev"))

    assertEquals(1, result.success.size)
    assertEquals("capture", result.success[0].providerName)
    assertEquals(0, result.failed.size)
    assertTrue(result.partialSuccess)
  }

  @Test
  fun `trackDetailed returns failure when provider throws`() = runTest {
    val client = Client(ClientOptions(providers = listOf(FailingProvider())))
    val result = client.trackDetailed(Event("Ev"))

    assertEquals(1, result.failed.size)
    assertEquals("provider down", result.failed[0].error?.message)
    assertFalse(result.partialSuccess)
  }

  @Test
  fun `trackDetailed partial success when one of two fails`() = runTest {
    val good = CaptureProvider("good")
    val client = Client(ClientOptions(providers = listOf(good, FailingProvider())))
    val result = client.trackDetailed(Event("Ev"))

    assertTrue(result.partialSuccess)
    assertEquals(1, result.success.size)
    assertEquals(1, result.failed.size)
  }

  @Test
  fun `global context is lowest priority`() = runTest {
    setGlobalContext(AnalyticsContext(userId = "global-user"))
    val p = CaptureProvider()
    val client = Client(ClientOptions(providers = listOf(p)))
    client.track(Event("Ev"))
    assertEquals("global-user", p.tracked[0].userId)
  }

  @Test
  fun `client context overrides global context`() = runTest {
    setGlobalContext(AnalyticsContext(userId = "global-user"))
    val p = CaptureProvider()
    val client =
        Client(
            ClientOptions(
                providers = listOf(p), context = AnalyticsContext(userId = "client-user")))
    client.track(Event("Ev"))
    assertEquals("client-user", p.tracked[0].userId)
  }

  @Test
  fun `transaction context is level 2`() = runTest {
    val p = CaptureProvider()
    val base = Client(ClientOptions(providers = listOf(p)))
    val scoped =
        base.withTransaction(AnalyticsContext(userId = "tx-user", anonymousId = "anon-123"))
    scoped.track(Event("Ev"))
    assertEquals("tx-user", p.tracked[0].userId)
    assertEquals("anon-123", p.tracked[0].anonymousId)
  }

  @Test
  fun `client context (level 3) overrides transaction (level 2)`() = runTest {
    setGlobalContext(AnalyticsContext(userId = "global-user"))
    val p = CaptureProvider()
    val base =
        Client(
            ClientOptions(
                providers = listOf(p), context = AnalyticsContext(userId = "client-user")))
    val scoped = base.withTransaction(AnalyticsContext(userId = "tx-user"))
    scoped.track(Event("Ev"))
    assertEquals("client-user", p.tracked[0].userId)
  }

  @Test
  fun `invocation override (level 4) wins over all`() = runTest {
    setGlobalContext(AnalyticsContext(userId = "global-user"))
    val p = CaptureProvider()
    val client =
        Client(
            ClientOptions(
                providers = listOf(p), context = AnalyticsContext(userId = "client-user")))
    client.track(
        Event("Ev"), TrackOptions(contextOverride = AnalyticsContext(userId = "invocation-user")))
    assertEquals("invocation-user", p.tracked[0].userId)
  }

  @Test
  fun `before hook can mutate event name`() = runTest {
    val p = CaptureProvider()

    class RenameHook : UnimplementedHook() {
      override suspend fun before(hc: HookContext, hints: HookHints) =
          EventEnvelope("Renamed", emptyMap(), hc.context)
    }

    val client = Client(ClientOptions(providers = listOf(p), hooks = listOf(RenameHook())))
    client.track(Event("Original"))
    assertEquals("Renamed", p.tracked[0].eventName)
  }

  @Test
  fun `before hook cancellation prevents dispatch`() = runTest {
    val p = CaptureProvider()

    class CancelHook : UnimplementedHook() {
      override suspend fun before(hc: HookContext, hints: HookHints): EventEnvelope? =
          throw Exception("event cancelled by hook")
    }

    val client = Client(ClientOptions(providers = listOf(p), hooks = listOf(CancelHook())))
    assertFailsWith<Exception>("event cancelled by hook") { client.track(Event("Ev")) }
    assertEquals(0, p.tracked.size)
  }

  @Test
  fun `identify dispatches with userId and traits`() = runTest {
    val p = CaptureProvider()
    val client = Client(ClientOptions(providers = listOf(p)))
    client.identify("user-1", mapOf("email" to "a@b.com"))

    assertEquals(1, p.identified.size)
    assertEquals("user-1", p.identified[0].userId)
    assertEquals("a@b.com", p.identified[0].traits["email"])
  }

  @Test
  fun `group dispatches with groupId and traits`() = runTest {
    val p = CaptureProvider()
    val client = Client(ClientOptions(providers = listOf(p)))
    client.group("org-1", mapOf("plan" to "pro"))

    assertEquals(1, p.grouped.size)
    assertEquals("org-1", p.grouped[0].groupId)
    assertEquals("pro", p.grouped[0].traits["plan"])
  }

  @Test
  fun `page dispatches with name and properties`() = runTest {
    val p = CaptureProvider()
    val client = Client(ClientOptions(providers = listOf(p)))
    client.page("/home", mapOf("referrer" to "google"))

    assertEquals(1, p.paged.size)
    assertEquals("/home", p.paged[0].name)
    assertEquals("google", p.paged[0].properties["referrer"])
  }

  @Test
  fun `alias dispatches with userId and previousId`() = runTest {
    val p = CaptureProvider()
    val client = Client(ClientOptions(providers = listOf(p)))
    client.alias("new-user", "anon-prev")

    assertEquals(1, p.aliased.size)
    assertEquals("new-user", p.aliased[0].userId)
    assertEquals("anon-prev", p.aliased[0].previousId)
  }

  @Test
  fun `setContext updates client context after construction`() = runTest {
    val p = CaptureProvider()
    val client = Client(ClientOptions(providers = listOf(p)))
    client.setContext(AnalyticsContext(userId = "updated"))
    client.track(Event("Ev"))
    assertEquals("updated", p.tracked[0].userId)
  }
}
