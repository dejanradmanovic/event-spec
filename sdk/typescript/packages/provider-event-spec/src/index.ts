import { ErrUnsupportedOperation } from '@dejanradmanovic/event-spec-api';
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
import type { ServerProviderConfig } from './config';

export type { ServerProviderConfig } from './config';

const VERSION = '1.0.0';

// EventSpecProvider is a thin-client Provider that forwards every analytics call
// to an event-spec runtime ingestion server over HTTP.
export class EventSpecProvider implements Provider {
  private readonly baseURL: string;
  private readonly apiKey: string;
  private readonly source: string;
  private readonly maxRetries: number;
  private readonly initialBackoffMs: number;
  private readonly maxBackoffMs: number;
  private readonly multiplier: number;
  private readonly jitter: boolean;
  private readonly rateLimitIntervalMs: number;
  private nextSendTime = 0;
  private closed = false;

  constructor(config: ServerProviderConfig) {
    const retry = config.retryConfig ?? {};
    const rateLimit = config.rateLimitConfig ?? {};

    this.baseURL = resolveBaseURL(config);
    this.apiKey = config.apiKey;
    this.source = config.source;
    this.maxRetries = retry.maxRetries ?? 3;
    this.initialBackoffMs = retry.initialBackoffMs ?? 100;
    this.maxBackoffMs = retry.maxBackoffMs ?? 30_000;
    this.multiplier = retry.multiplier ?? 2.0;
    this.jitter = retry.jitter ?? true;
    this.rateLimitIntervalMs =
      rateLimit.requestsPerSecond && rateLimit.requestsPerSecond > 0
        ? Math.ceil(1000 / rateLimit.requestsPerSecond)
        : 0;
  }

  metadata(): ProviderMetadata {
    return {
      name: 'event-spec',
      version: VERSION,
      capabilities: { track: true, identify: true, group: true, page: true, alias: true },
    };
  }

  hooks(): Hook[] {
    return [];
  }

  async track(msg: TrackMessage): Promise<void> {
    await this.post('/v1/track', {
      source: this.source,
      event_name: msg.eventName,
      properties: msg.properties,
      context: buildContext(msg),
      timestamp: msg.timestamp.toISOString(),
    });
  }

  async identify(msg: IdentifyMessage): Promise<void> {
    await this.post('/v1/identify', {
      source: this.source,
      user_id: msg.userId,
      anonymous_id: msg.anonymousId,
      traits: msg.traits,
      timestamp: msg.timestamp.toISOString(),
    });
  }

  async group(msg: GroupMessage): Promise<void> {
    await this.post('/v1/group', {
      source: this.source,
      user_id: msg.userId,
      anonymous_id: msg.anonymousId,
      group_id: msg.groupId,
      traits: msg.traits,
      timestamp: msg.timestamp.toISOString(),
    });
  }

  async page(msg: PageMessage): Promise<void> {
    await this.post('/v1/page', {
      source: this.source,
      user_id: msg.userId,
      anonymous_id: msg.anonymousId,
      name: msg.name,
      properties: msg.properties,
      timestamp: msg.timestamp.toISOString(),
    });
  }

  async alias(msg: AliasMessage): Promise<void> {
    await this.post('/v1/alias', {
      source: this.source,
      user_id: msg.userId,
      previous_id: msg.previousId,
      timestamp: msg.timestamp.toISOString(),
    });
  }

  async flush(): Promise<void> {
    if (this.closed) throw new Error('event-spec provider: already shut down');
    await this.post('/v1/flush', { source: this.source });
  }

  async shutdown(): Promise<void> {
    if (this.closed) throw new Error('event-spec provider: already shut down');
    await this.post('/v1/flush', { source: this.source });
    this.closed = true;
  }

  private async post(path: string, body: unknown): Promise<void> {
    if (this.closed) throw new Error('event-spec provider: already shut down');

    // Rate limiting: reserve a send slot and delay until it opens.
    if (this.rateLimitIntervalMs > 0) {
      const now = Date.now();
      const waitUntil = Math.max(now, this.nextSendTime);
      this.nextSendTime = waitUntil + this.rateLimitIntervalMs;
      if (waitUntil > now) {
        await sleep(waitUntil - now);
      }
    }

    const serialized = JSON.stringify(body);
    let lastError: Error | undefined;
    let backoff = this.initialBackoffMs;

    for (let attempt = 0; attempt <= this.maxRetries; attempt++) {
      if (attempt > 0) {
        const delay = this.jitter ? backoff * (0.5 + Math.random() * 0.5) : backoff;
        await sleep(delay);
        backoff = Math.min(backoff * this.multiplier, this.maxBackoffMs);
      }
      try {
        const resp = await fetch(this.baseURL + path, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Accept: 'application/json',
            Authorization: `Bearer ${this.apiKey}`,
          },
          body: serialized,
        });
        if (!resp.ok) {
          throw new Error(`event-spec provider: unexpected status ${resp.status} from ${path}`);
        }
        return;
      } catch (err) {
        lastError = err instanceof Error ? err : new Error(String(err));
        if (attempt === this.maxRetries) break;
      }
    }
    throw lastError ?? new Error(`event-spec provider: request to ${path} failed`);
  }
}

// resolveBaseURL applies proxy rewriting from config to the baseURL.
function resolveBaseURL(config: ServerProviderConfig): string {
  const base = config.baseURL;
  if (!config.proxyUrl) return base;
  switch (config.proxyMode) {
    case 'reverse_proxy': {
      try {
        const proxy = new URL(config.proxyUrl);
        const target = new URL(base);
        target.protocol = proxy.protocol;
        target.host = proxy.host;
        if (proxy.pathname && proxy.pathname !== '/') {
          const basePath = proxy.pathname.replace(/\/$/, '');
          const provPath = target.pathname.replace(/^\//, '');
          target.pathname = `${basePath}/${provPath}`;
        }
        return target.toString().replace(/\/$/, '');
      } catch {
        return base;
      }
    }
    case 'custom':
      return config.proxyUrl.replace(/\/$/, '');
    default:
      return base;
  }
}

function buildContext(
  msg: TrackMessage | IdentifyMessage | GroupMessage | PageMessage,
): Record<string, unknown> {
  const ctx: Record<string, unknown> = {
    user_id: 'userId' in msg ? msg.userId : undefined,
    anonymous_id: 'anonymousId' in msg ? msg.anonymousId : undefined,
  };
  const attrs: Record<string, unknown> = {};
  if (msg.context?.userAgent) attrs['user_agent'] = msg.context.userAgent;
  if (msg.context?.ipAddress) attrs['ip_address'] = msg.context.ipAddress;
  if (msg.context?.extra) Object.assign(attrs, msg.context.extra);
  if (Object.keys(attrs).length > 0) ctx['attributes'] = attrs;
  return ctx;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// Re-export ErrUnsupportedOperation for consumers that need it.
export { ErrUnsupportedOperation };
