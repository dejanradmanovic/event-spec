export type OverflowPolicy = 'drop_oldest' | 'drop_newest';

export interface QueueOptions {
  maxSize?: number;
  batchSize?: number;
  flushIntervalMs?: number;
  overflowPolicy?: OverflowPolicy;
}

export type FlushCallback<T> = (items: T[]) => Promise<void>;

// EventQueue is a generic event buffer with configurable batching and overflow policies.
// Provider implementations use this to buffer events before sending them in batches.
export class EventQueue<T> {
  private readonly items: T[] = [];
  private readonly maxSize: number;
  private readonly batchSize: number;
  private readonly flushIntervalMs: number;
  private readonly overflowPolicy: OverflowPolicy;
  private readonly onFlush: FlushCallback<T>;
  private timer: ReturnType<typeof setInterval> | undefined;

  constructor(onFlush: FlushCallback<T>, opts: QueueOptions = {}) {
    this.onFlush = onFlush;
    this.maxSize = opts.maxSize ?? 10000;
    this.batchSize = opts.batchSize ?? 100;
    this.flushIntervalMs = opts.flushIntervalMs ?? 10000;
    this.overflowPolicy = opts.overflowPolicy ?? 'drop_oldest';

    if (this.flushIntervalMs > 0) {
      this.timer = setInterval(() => {
        void this.flushAll();
      }, this.flushIntervalMs);
    }
  }

  enqueue(item: T): void {
    if (this.items.length >= this.maxSize) {
      if (this.overflowPolicy === 'drop_oldest') {
        this.items.shift();
      } else {
        return;
      }
    }
    this.items.push(item);
    if (this.items.length >= this.batchSize) {
      void this.flush();
    }
  }

  async flush(): Promise<void> {
    if (this.items.length === 0) return;
    const batch = this.items.splice(0, this.batchSize);
    await this.onFlush(batch);
  }

  async flushAll(): Promise<void> {
    while (this.items.length > 0) {
      await this.flush();
    }
  }

  async shutdown(): Promise<void> {
    if (this.timer !== undefined) {
      clearInterval(this.timer);
      this.timer = undefined;
    }
    await this.flushAll();
  }

  get size(): number {
    return this.items.length;
  }
}
