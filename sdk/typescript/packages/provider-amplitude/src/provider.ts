import { EventQueue, ErrUnsupportedOperation } from '@event-spec/api';
import type {
  Provider,
  ProviderMetadata,
  Hook,
  TrackMessage,
  IdentifyMessage,
  GroupMessage,
  PageMessage,
  AliasMessage,
} from '@event-spec/api';
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
  private readonly queue: EventQueue<TrackMessage>;

  constructor(config: AmplitudeConfig) {
    this.apiKey = config.apiKey;
    this.endpoint = config.endpoint ?? DEFAULT_ENDPOINT;
    this.maxRetries = config.maxRetries ?? 3;
    this.queue = new EventQueue<TrackMessage>((items) => this.flushBatch(items), {
      batchSize: config.batchSize ?? 100,
      flushIntervalMs: config.flushIntervalMs ?? 10000,
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

    let lastError: Error | undefined;
    for (let attempt = 0; attempt <= this.maxRetries; attempt++) {
      if (attempt > 0) {
        await sleep(Math.pow(2, attempt - 1) * 1000);
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

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
