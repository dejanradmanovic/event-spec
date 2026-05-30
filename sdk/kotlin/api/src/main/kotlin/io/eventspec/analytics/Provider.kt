package io.eventspec.analytics

import java.time.Instant

data class ProviderCapabilities(
    val track: Boolean = true,
    val identify: Boolean = true,
    val group: Boolean = true,
    val page: Boolean = true,
    val alias: Boolean = true,
)

data class ProviderMetadata(
    val name: String,
    val version: String,
    val capabilities: ProviderCapabilities = ProviderCapabilities(),
)

data class MessageContext(
    val ipAddress: String? = null,
    val userAgent: String? = null,
    val locale: String? = null,
    val timezone: String? = null,
    val library: Map<String, Any>? = null,
    val app: Map<String, Any>? = null,
    val device: Map<String, Any>? = null,
    val os: Map<String, Any>? = null,
    val network: Map<String, Any>? = null,
    val screen: Map<String, Any>? = null,
    val extra: Map<String, Any>? = null,
)

data class TrackMessage(
    val messageId: String,
    val timestamp: Instant,
    val eventName: String,
    val properties: Map<String, Any?>,
    val userId: String,
    val anonymousId: String,
    val context: MessageContext = MessageContext(),
)

data class IdentifyMessage(
    val messageId: String,
    val timestamp: Instant,
    val userId: String,
    val anonymousId: String,
    val traits: Map<String, Any?>,
    val context: MessageContext = MessageContext(),
)

data class GroupMessage(
    val messageId: String,
    val timestamp: Instant,
    val userId: String,
    val anonymousId: String,
    val groupId: String,
    val traits: Map<String, Any?>,
    val context: MessageContext = MessageContext(),
)

data class PageMessage(
    val messageId: String,
    val timestamp: Instant,
    val userId: String,
    val anonymousId: String,
    val name: String,
    val properties: Map<String, Any?>,
    val context: MessageContext = MessageContext(),
)

data class AliasMessage(
    val messageId: String,
    val timestamp: Instant,
    val userId: String,
    val previousId: String,
    val context: MessageContext = MessageContext(),
)

class UnsupportedOperationException(operation: String) :
    Exception("unsupported provider operation: $operation")

interface Provider {
  fun metadata(): ProviderMetadata

  fun hooks(): List<Hook> = emptyList()

  suspend fun track(msg: TrackMessage)

  suspend fun identify(msg: IdentifyMessage)

  suspend fun group(msg: GroupMessage)

  suspend fun page(msg: PageMessage)

  suspend fun alias(msg: AliasMessage)

  suspend fun flush()

  suspend fun shutdown()
}
