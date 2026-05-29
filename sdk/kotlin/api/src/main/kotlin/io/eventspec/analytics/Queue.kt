package io.eventspec.analytics

import kotlinx.coroutines.*
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

enum class OverflowPolicy {
  DROP_OLDEST,
  DROP_NEWEST
}

data class QueueOptions(
    val maxSize: Int = 10_000,
    val batchSize: Int = 100,
    val flushIntervalMs: Long = 10_000,
    val overflowPolicy: OverflowPolicy = OverflowPolicy.DROP_OLDEST,
)

// EventQueue buffers events and flushes them in batches.
class EventQueue<T>(
    private val onFlush: suspend (List<T>) -> Unit,
    private val opts: QueueOptions = QueueOptions(),
    scope: CoroutineScope = CoroutineScope(Dispatchers.Default),
) {
  private val items = ArrayDeque<T>()
  private val mutex = Mutex()
  private var timer: Job? = null

  init {
    if (opts.flushIntervalMs > 0) {
      timer =
          scope.launch {
            while (isActive) {
              delay(opts.flushIntervalMs)
              flushAll()
            }
          }
    }
  }

  suspend fun enqueue(item: T) {
    mutex.withLock {
      if (items.size >= opts.maxSize) {
        when (opts.overflowPolicy) {
          OverflowPolicy.DROP_OLDEST -> items.removeFirst()
          OverflowPolicy.DROP_NEWEST -> return
        }
      }
      items.addLast(item)
    }
    if (size() >= opts.batchSize) flush()
  }

  suspend fun flush() {
    val batch =
        mutex.withLock {
          if (items.isEmpty()) return
          val b = items.take(opts.batchSize).toList()
          repeat(b.size) { items.removeFirst() }
          b
        }
    onFlush(batch)
  }

  suspend fun flushAll() {
    while (size() > 0) flush()
  }

  suspend fun shutdown() {
    timer?.cancel()
    flushAll()
  }

  fun size(): Int = items.size
}
