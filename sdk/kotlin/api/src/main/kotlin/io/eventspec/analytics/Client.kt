package io.eventspec.analytics

import java.time.Instant
import java.util.UUID
import kotlinx.coroutines.*

data class Event(
    val name: String,
    val properties: Map<String, Any?> = emptyMap(),
)

data class TrackOptions(
    val contextOverride: AnalyticsContext? = null,
)

data class ProviderResult(
    val providerName: String,
    val error: Throwable? = null,
    val latencyMs: Long = 0,
)

data class DispatchResult(
    val success: List<ProviderResult>,
    val failed: List<ProviderResult>,
) {
  val partialSuccess: Boolean
    get() = success.isNotEmpty()
}

private var globalContext = AnalyticsContext()
private var globalProviders: List<Provider> = emptyList()
private val globalHooks = mutableListOf<Hook>()

fun setGlobalContext(ctx: AnalyticsContext) {
  globalContext = ctx
}

fun setGlobalProvider(vararg providers: Provider) {
  globalProviders = providers.toList()
}

fun addGlobalHooks(vararg hooks: Hook) {
  globalHooks.addAll(hooks)
}

data class ClientOptions(
    val providers: List<Provider> = emptyList(),
    val hooks: List<Hook> = emptyList(),
    val context: AnalyticsContext = AnalyticsContext(),
)

class Client(private var options: ClientOptions = ClientOptions()) {
  private var clientCtx: AnalyticsContext = options.context
  private var transactionCtx: AnalyticsContext? = null

  fun setContext(ctx: AnalyticsContext) {
    clientCtx = ctx
  }

  fun withTransaction(tx: AnalyticsContext): Client {
    val clone = Client(options)
    clone.clientCtx = clientCtx
    clone.transactionCtx = tx
    return clone
  }

  private fun mergedContext(override: AnalyticsContext?): AnalyticsContext {
    var ctx = merge(globalContext, transactionCtx ?: AnalyticsContext())
    ctx = merge(ctx, clientCtx)
    if (override != null) ctx = merge(ctx, override)
    return ctx
  }

  private fun allHooks(providers: List<Provider>): List<Hook> {
    val providerHooks = providers.flatMap { it.hooks() }
    return globalHooks + options.hooks + providerHooks
  }

  private fun activeProviders(): List<Provider> =
      if (options.providers.isNotEmpty()) options.providers else globalProviders

  suspend fun track(event: Event, opts: TrackOptions? = null): Unit {
    trackDetailed(event, opts)
  }

  suspend fun trackDetailed(event: Event, opts: TrackOptions? = null): DispatchResult {
    val providers = activeProviders()
    val chain = HookChain(allHooks(providers))
    val mergedCtx = mergedContext(opts?.contextOverride)

    val hc = HookContext(operation = "track", eventName = event.name, context = mergedCtx)
    val envelope = chain.before(hc)

    val resolvedName = envelope?.eventName ?: event.name
    val resolvedProps = envelope?.properties ?: event.properties

    val msg =
        TrackMessage(
            messageId = UUID.randomUUID().toString(),
            timestamp = Instant.now(),
            eventName = resolvedName,
            properties = resolvedProps,
            userId = mergedCtx.userId,
            anonymousId = mergedCtx.anonymousId,
        )

    return dispatchTrack(providers, chain, hc, msg)
  }

  private suspend fun dispatchTrack(
      providers: List<Provider>,
      chain: HookChain,
      hc: HookContext,
      msg: TrackMessage,
  ): DispatchResult = coroutineScope {
    val results =
        providers
            .map { p ->
              async {
                val start = System.currentTimeMillis()
                runCatching { p.track(msg) }
                    .fold(
                        onSuccess = {
                          val r =
                              ProviderResult(
                                  p.metadata().name, latencyMs = System.currentTimeMillis() - start)
                          val pr =
                              HookContext(
                                  hc.operation, hc.eventName, hc.context, msg, p.metadata().name)
                          chain.after(pr, HookResult(delivered = true, dropped = false))
                          chain.finally(pr, HookResult(delivered = true, dropped = false))
                          r
                        },
                        onFailure = { err ->
                          val r =
                              ProviderResult(
                                  p.metadata().name,
                                  error = err,
                                  latencyMs = System.currentTimeMillis() - start)
                          val pr =
                              HookContext(
                                  hc.operation, hc.eventName, hc.context, msg, p.metadata().name)
                          chain.error(pr, err)
                          chain.finally(
                              pr, HookResult(delivered = false, dropped = false, error = err))
                          r
                        },
                    )
              }
            }
            .awaitAll()

    val success = results.filter { it.error == null }
    val failed = results.filter { it.error != null }
    DispatchResult(success, failed)
  }

  suspend fun identify(
      userId: String,
      traits: Map<String, Any?> = emptyMap(),
      opts: TrackOptions? = null
  ) {
    val mergedCtx = mergedContext(opts?.contextOverride)
    val msg =
        IdentifyMessage(
            messageId = UUID.randomUUID().toString(),
            timestamp = Instant.now(),
            userId = userId,
            anonymousId = mergedCtx.anonymousId,
            traits = traits,
        )
    coroutineScope { activeProviders().map { p -> async { p.identify(msg) } }.awaitAll() }
  }

  suspend fun group(
      groupId: String,
      traits: Map<String, Any?> = emptyMap(),
      opts: TrackOptions? = null
  ) {
    val mergedCtx = mergedContext(opts?.contextOverride)
    val msg =
        GroupMessage(
            messageId = UUID.randomUUID().toString(),
            timestamp = Instant.now(),
            userId = mergedCtx.userId,
            anonymousId = mergedCtx.anonymousId,
            groupId = groupId,
            traits = traits,
        )
    coroutineScope { activeProviders().map { p -> async { p.group(msg) } }.awaitAll() }
  }

  suspend fun page(
      name: String,
      properties: Map<String, Any?> = emptyMap(),
      opts: TrackOptions? = null
  ) {
    val mergedCtx = mergedContext(opts?.contextOverride)
    val msg =
        PageMessage(
            messageId = UUID.randomUUID().toString(),
            timestamp = Instant.now(),
            userId = mergedCtx.userId,
            anonymousId = mergedCtx.anonymousId,
            name = name,
            properties = properties,
        )
    coroutineScope { activeProviders().map { p -> async { p.page(msg) } }.awaitAll() }
  }

  suspend fun alias(userId: String, previousId: String, opts: TrackOptions? = null) {
    val mergedCtx = mergedContext(opts?.contextOverride)
    val msg =
        AliasMessage(
            messageId = UUID.randomUUID().toString(),
            timestamp = Instant.now(),
            userId = userId,
            previousId = previousId,
        )
    coroutineScope { activeProviders().map { p -> async { p.alias(msg) } }.awaitAll() }
  }

  suspend fun flush() {
    coroutineScope { activeProviders().map { p -> async { p.flush() } }.awaitAll() }
  }
}
