import { EventQueue, ErrUnsupportedOperation } from '@dejanradmanovic/event-spec-api';
import type {
  Provider,
  ProviderMetadata,
  Hook,
  TrackMessage,
  IdentifyMessage,
  GroupMessage,
  PageMessage,
  AliasMessage,
} from '@dejanradmanovic/event-spec-api';
import type { AmplitudeConfig } from './config';
import { DEFAULT_ENDPOINT } from './config';
import { mapTrackMessage, mapIdentifyMessage, mapGroupMessage, mapAliasMessage } from './mapper';
import type { AmplitudeEvent } from './mapper';

// AmplitudeProvider sends analytics events to Amplitude via the batch HTTP API.
// Capabilities: Track ✅, Identify ✅, Group ✅, Page ❌ (unsupported), Alias ✅.
export class AmplitudeProvider implements Provider {
  private readonly apiKey: string;
  private readonly endpoint: string;
  private readonly maxRetries: number;
  private readonly initialBackoffMs: number;
  private readonly maxBackoffMs: number;
  private readonly retryMultiplier: number;
  private readonly jitter: boolean;
  private readonly rateLimitIntervalMs: number;
  private nextSendTime = 0;
  private readonly queue: EventQueue<TrackMessage>;

  constructor(config: AmplitudeConfig) {
    this.apiKey = config.apiKey;
    this.endpoint = resolveEndpoint(config);
    this.maxRetries = config.maxRetries ?? 3;
    this.initialBackoffMs = config.initialBackoffMs ?? 100;
    this.maxBackoffMs = config.maxBackoffMs ?? 30_000;
    this.retryMultiplier = config.retryMultiplier ?? 2.0;
    this.jitter = config.jitter ?? true;
    this.rateLimitIntervalMs =
      config.requestsPerSecond && config.requestsPerSecond > 0
        ? Math.ceil(1000 / config.requestsPerSecond)
        : 0;
    this.queue = new EventQueue<TrackMessage>((items) => this.flushBatch(items), {
      batchSize: config.batchSize ?? 100,
      flushIntervalMs: config.flushIntervalMs ?? 10000,
      maxSize: config.maxQueueSize ?? 10000,
      overflowPolicy: config.overflowPolicy ?? 'drop_oldest',
    });
  }

  metadata(): ProviderMetadata {
    return {
      name: 'amplitude',
      version: '0.1.0',
      capabilities: { track: true, identify: true, group: true, page: false, alias: true },
    };
  }

  hooks(): Hook[] {
    return [];
  }

  // track enqueues a track event for batched delivery.
  async track(msg: TrackMessage): Promise<void> {
    this.queue.enqueue(msg);
  }

  // identify sends a $identify event synchronously so user-profile state arrives at
  // Amplitude before any subsequent Track events that depend on it.
  async identify(msg: IdentifyMessage): Promise<void> {
    await this.sendBatch([mapIdentifyMessage(msg)]);
  }

  // group sends a $groupidentify event synchronously for the same reason as identify.
  async group(msg: GroupMessage): Promise<void> {
    await this.sendBatch([mapGroupMessage(msg)]);
  }

  // page is unsupported — Amplitude has no native page concept.
  async page(_msg: PageMessage): Promise<void> {
    throw ErrUnsupportedOperation;
  }

  // alias sends a $identify event synchronously, linking previousId to userId.
  async alias(msg: AliasMessage): Promise<void> {
    await this.sendBatch([mapAliasMessage(msg)]);
  }

  async flush(): Promise<void> {
    await this.queue.flushAll();
  }

  async shutdown(): Promise<void> {
    await this.queue.shutdown();
  }

  private async flushBatch(messages: TrackMessage[]): Promise<void> {
    const events = messages.map(mapTrackMessage);
    await this.sendBatch(events);
  }

  private async sendBatch(events: AmplitudeEvent[]): Promise<void> {
    const body = JSON.stringify({ api_key: this.apiKey, events });

    // Rate limiting: reserve a send slot and delay until it opens.
    if (this.rateLimitIntervalMs > 0) {
      const now = Date.now();
      const waitUntil = Math.max(now, this.nextSendTime);
      this.nextSendTime = waitUntil + this.rateLimitIntervalMs;
      if (waitUntil > now) {
        await sleep(waitUntil - now);
      }
    }

    let lastError: Error | undefined;
    let backoff = this.initialBackoffMs;
    for (let attempt = 0; attempt <= this.maxRetries; attempt++) {
      if (attempt > 0) {
        const delay = this.jitter ? backoff * (0.5 + Math.random() * 0.5) : backoff;
        await sleep(delay);
        backoff = Math.min(backoff * this.retryMultiplier, this.maxBackoffMs);
      }
      try {
        const resp = await fetch(this.endpoint, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
          body,
        });
        if (!resp.ok) {
          throw new Error(`amplitude: unexpected status ${resp.status}`);
        }
        return;
      } catch (err) {
        lastError = err instanceof Error ? err : new Error(String(err));
        if (attempt === this.maxRetries) break;
      }
    }
    throw lastError ?? new Error('amplitude: send failed');
  }
}

// resolveEndpoint computes the effective HTTP endpoint from config, applying proxy rewriting.
function resolveEndpoint(config: AmplitudeConfig): string {
  const base = config.endpoint ?? DEFAULT_ENDPOINT;
  if (!config.proxyUrl) return base;
  switch (config.proxyMode) {
    case 'reverse_proxy': {
      try {
        const proxy = new URL(config.proxyUrl);
        const target = new URL(base);
        target.protocol = proxy.protocol;
        target.host = proxy.host; // includes port when non-default
        if (proxy.pathname && proxy.pathname !== '/') {
          const basePath = proxy.pathname.replace(/\/$/, '');
          const provPath = target.pathname.replace(/^\//, '');
          target.pathname = `${basePath}/${provPath}`;
        }
        return target.toString();
      } catch {
        return base;
      }
    }
    case 'custom':
      return config.proxyUrl;
    default:
      return base;
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
