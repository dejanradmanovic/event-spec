import { type AnalyticsContext, merge } from './context';
import {
  HookChain,
  type Hook,
  type HookContext,
  type HookHints,
  type EventEnvelope,
} from './hooks';
import type {
  Provider,
  MessageContext,
  TrackMessage,
  IdentifyMessage,
  GroupMessage,
  PageMessage,
  AliasMessage,
} from './provider';

// Global state — shared across all Client instances (level-1 context chain).
let _globalContext: AnalyticsContext = {};
let _globalHooks: Hook[] = [];
let _globalClient: Client | null = null;

function getGlobalContext(): AnalyticsContext {
  return {
    ..._globalContext,
    attributes: _globalContext.attributes ? { ..._globalContext.attributes } : undefined,
  };
}

function getGlobalHooks(): Hook[] {
  return [..._globalHooks];
}

export type DeliveryState = 'accepted' | 'delivered' | 'failed' | 'dropped';

export interface ProviderResult {
  providerName: string;
  state: DeliveryState;
  error?: Error;
  latencyMs: number;
}

export interface DispatchResult {
  success: ProviderResult[];
  failed: ProviderResult[];
  partialSuccess: boolean;
}

export interface Event {
  name: string;
  properties?: Record<string, unknown>;
}

export interface TrackOptions {
  contextOverride?: AnalyticsContext;
  hints?: HookHints;
}

export interface TransactionContext {
  userId?: string;
  anonymousId?: string;
  attributes?: Record<string, unknown>;
}

export interface ClientOptions {
  providers?: Provider[];
  context?: AnalyticsContext;
  hooks?: Hook[];
}

type ProviderFn = (p: Provider, msgId: string, ts: Date, env: EventEnvelope) => Promise<void>;

export class Client {
  private providers: Provider[];
  private clientCtx: AnalyticsContext;
  private readonly clientHooks: Hook[];
  private txCtx: TransactionContext | undefined;

  constructor(opts: ClientOptions = {}) {
    this.providers = opts.providers ? [...opts.providers] : [];
    this.clientCtx = opts.context ?? {};
    this.clientHooks = opts.hooks ? [...opts.hooks] : [];
  }

  setProviders(...providers: Provider[]): void {
    this.providers = [...providers];
  }

  addProvider(...providers: Provider[]): void {
    this.providers = [...this.providers, ...providers];
  }

  setContext(ctx: AnalyticsContext): void {
    this.clientCtx = ctx;
  }

  // withTransaction returns a shallow-cloned Client with txCtx bound at level 2.
  withTransaction(tx: TransactionContext): Client {
    const clone = new Client({
      providers: this.providers,
      context: this.clientCtx,
      hooks: this.clientHooks,
    });
    clone.txCtx = tx;
    return clone;
  }

  async track(event: Event, opts?: TrackOptions): Promise<void> {
    await this.trackDetailed(event, opts);
  }

  async trackDetailed(event: Event, opts?: TrackOptions): Promise<DispatchResult> {
    return this.dispatchAll(
      'track',
      event.name,
      event.properties ?? {},
      opts,
      async (p, msgId, ts, env) => {
        const msg: TrackMessage = {
          messageId: msgId,
          timestamp: ts,
          eventName: env.eventName,
          properties: env.properties,
          userId: env.context.userId ?? '',
          anonymousId: env.context.anonymousId ?? '',
          context: buildMessageContext(env.context),
        };
        await p.track(msg);
      },
    );
  }

  async identify(
    userId: string,
    traits: Record<string, unknown>,
    opts?: TrackOptions,
  ): Promise<void> {
    await this.dispatchAll('identify', '$identify', traits, opts, async (p, msgId, ts, env) => {
      const uid = env.context.userId || userId;
      const msg: IdentifyMessage = {
        messageId: msgId,
        timestamp: ts,
        userId: uid,
        anonymousId: env.context.anonymousId ?? '',
        traits: env.properties,
        context: buildMessageContext(env.context),
      };
      await p.identify(msg);
    });
  }

  async group(
    groupId: string,
    traits: Record<string, unknown>,
    opts?: TrackOptions,
  ): Promise<void> {
    const props = { ...traits, group_id: groupId };
    await this.dispatchAll('group', '$group', props, opts, async (p, msgId, ts, env) => {
      const gid =
        typeof env.properties['group_id'] === 'string' ? env.properties['group_id'] : groupId;
      const msg: GroupMessage = {
        messageId: msgId,
        timestamp: ts,
        userId: env.context.userId ?? '',
        anonymousId: env.context.anonymousId ?? '',
        groupId: gid,
        traits: env.properties,
        context: buildMessageContext(env.context),
      };
      await p.group(msg);
    });
  }

  async page(name: string, props: Record<string, unknown>, opts?: TrackOptions): Promise<void> {
    const pageProps = { ...props, name };
    await this.dispatchAll('page', '$page', pageProps, opts, async (p, msgId, ts, env) => {
      const pname = typeof env.properties['name'] === 'string' ? env.properties['name'] : name;
      const msg: PageMessage = {
        messageId: msgId,
        timestamp: ts,
        userId: env.context.userId ?? '',
        anonymousId: env.context.anonymousId ?? '',
        name: pname,
        properties: env.properties,
        context: buildMessageContext(env.context),
      };
      await p.page(msg);
    });
  }

  async alias(userId: string, previousId: string, opts?: TrackOptions): Promise<void> {
    await this.dispatchAll('alias', '$alias', {}, opts, async (p, msgId, ts, env) => {
      const msg: AliasMessage = {
        messageId: msgId,
        timestamp: ts,
        userId,
        previousId,
        context: buildMessageContext(env.context),
      };
      await p.alias(msg);
    });
  }

  async flush(): Promise<void> {
    const providers = [...this.providers];
    const errors: Error[] = [];
    for (const p of providers) {
      try {
        await p.flush();
      } catch (err) {
        errors.push(err instanceof Error ? err : new Error(String(err)));
      }
    }
    if (errors.length > 0) {
      throw new Error(`flush: ${errors.map((e) => e.message).join(', ')}`);
    }
  }

  async shutdown(): Promise<void> {
    const providers = [...this.providers];
    const errors: Error[] = [];
    for (const p of providers) {
      try {
        await p.shutdown();
      } catch (err) {
        errors.push(err instanceof Error ? err : new Error(String(err)));
      }
    }
    if (errors.length > 0) {
      throw new Error(`shutdown: ${errors.map((e) => e.message).join(', ')}`);
    }
  }

  // mergeContextChain applies the 4-level precedence chain:
  // global (1) → transaction/WithTransaction (2) → client (3) → invocation (4).
  private mergeContextChain(opts?: TrackOptions): AnalyticsContext {
    let result = getGlobalContext(); // level 1

    if (this.txCtx) {
      result = merge(result, this.txCtx); // level 2
    }

    result = merge(result, this.clientCtx); // level 3

    if (opts?.contextOverride) {
      result = merge(result, opts.contextOverride); // level 4
    }

    return result;
  }

  private collectAllHooks(): Hook[] {
    const apiHooks = getGlobalHooks();
    const clientHooks = [...this.clientHooks];
    const provHooks = this.providers.flatMap((p) => p.hooks());
    return [...apiHooks, ...clientHooks, ...provHooks];
  }

  private async dispatchAll(
    operation: 'track' | 'identify' | 'group' | 'page' | 'alias',
    eventName: string,
    properties: Record<string, unknown>,
    opts: TrackOptions | undefined,
    fn: ProviderFn,
  ): Promise<DispatchResult> {
    const merged = this.mergeContextChain(opts);
    const allHooks = this.collectAllHooks();
    const chain = new HookChain(allHooks);

    let env: EventEnvelope = {
      eventName,
      properties: { ...properties },
      context: merged,
    };

    let hc: HookContext = {
      operation,
      eventName,
      context: merged,
      message: env,
    };

    // Before runs once, gating ALL providers. A thrown error cancels dispatch.
    const mutated = await chain.before(hc, opts?.hints);
    if (mutated !== null) {
      env = mutated;
      hc = { ...hc, eventName: mutated.eventName, context: mutated.context, message: env };
    }

    const providers = [...this.providers];
    if (providers.length === 0) {
      return { success: [], failed: [], partialSuccess: false };
    }

    // Concurrent per-provider dispatch — mirrors Go's goroutine fan-out.
    const settled = await Promise.allSettled(
      providers.map(async (p): Promise<ProviderResult> => {
        const start = Date.now();
        const msgId = generateMessageId();
        const ts = new Date();
        const pName = p.metadata().name;
        const provHC: HookContext = { ...hc, provider: pName };

        try {
          await fn(p, msgId, ts, env);
          const result: ProviderResult = {
            providerName: pName,
            state: 'delivered',
            latencyMs: Date.now() - start,
          };
          const hookResult = { delivered: true, dropped: false };
          await chain.after(provHC, hookResult, opts?.hints).catch(() => {});
          chain.finally(provHC, hookResult, opts?.hints);
          return result;
        } catch (err) {
          const callErr = err instanceof Error ? err : new Error(String(err));
          const result: ProviderResult = {
            providerName: pName,
            state: 'failed',
            error: callErr,
            latencyMs: Date.now() - start,
          };
          const hookResult = { delivered: false, dropped: false, error: callErr };
          chain.error(provHC, callErr, opts?.hints);
          chain.finally(provHC, hookResult, opts?.hints);
          return result;
        }
      }),
    );

    const success: ProviderResult[] = [];
    const failed: ProviderResult[] = [];

    for (const r of settled) {
      const result =
        r.status === 'fulfilled'
          ? r.value
          : {
              providerName: 'unknown',
              state: 'failed' as DeliveryState,
              error: r.reason instanceof Error ? r.reason : new Error(String(r.reason)),
              latencyMs: 0,
            };

      if (result.error) {
        failed.push(result);
      } else {
        success.push(result);
      }
    }

    return { success, failed, partialSuccess: success.length > 0 };
  }
}

// Package-level global API — mirrors Go's analytics package functions.

export function setGlobalProvider(...providers: Provider[]): void {
  getOrCreateGlobalClient().setProviders(...providers);
}

export function addGlobalProvider(provider: Provider): void {
  getOrCreateGlobalClient().addProvider(provider);
}

export function setGlobalContext(ctx: AnalyticsContext): void {
  _globalContext = ctx;
}

export function addGlobalHooks(...hooks: Hook[]): void {
  _globalHooks = [..._globalHooks, ...hooks];
}

export function newClient(opts?: ClientOptions): Client {
  return new Client(opts);
}

export async function shutdown(): Promise<void> {
  if (_globalClient) {
    await _globalClient.shutdown();
  }
}

export async function track(event: Event, opts?: TrackOptions): Promise<void> {
  await getOrCreateGlobalClient().track(event, opts);
}

function getOrCreateGlobalClient(): Client {
  if (!_globalClient) {
    _globalClient = new Client();
  }
  return _globalClient;
}

function generateMessageId(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  // Fallback for environments without Web Crypto
  const bytes = new Uint8Array(16);
  if (typeof crypto !== 'undefined' && crypto.getRandomValues) {
    crypto.getRandomValues(bytes);
  } else {
    for (let i = 0; i < 16; i++) {
      bytes[i] = Math.floor(Math.random() * 256);
    }
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40; // version 4
  bytes[8] = (bytes[8] & 0x3f) | 0x80; // variant bits
  const hex = Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

// buildMessageContext promotes known AnalyticsContext attributes to typed MessageContext fields.
// Unknown attributes land in extra so arbitrary context_properties flow through.
function buildMessageContext(ctx: AnalyticsContext): MessageContext {
  if (!ctx.attributes || Object.keys(ctx.attributes).length === 0) {
    return {};
  }

  const mc: MessageContext = {};
  const extra: Record<string, unknown> = {};

  for (const [k, v] of Object.entries(ctx.attributes)) {
    switch (k) {
      case 'ip_address':
      case 'ip':
        if (typeof v === 'string') {
          mc.ipAddress = v;
        } else {
          extra[k] = v;
        }
        break;
      case 'user_agent':
        if (typeof v === 'string') {
          mc.userAgent = v;
        } else {
          extra[k] = v;
        }
        break;
      case 'locale':
        if (typeof v === 'string') {
          mc.locale = v;
        } else {
          extra[k] = v;
        }
        break;
      case 'timezone':
        if (typeof v === 'string') {
          mc.timezone = v;
        } else {
          extra[k] = v;
        }
        break;
      case 'library':
        if (isRecord(v)) {
          mc.library = v;
        } else {
          extra[k] = v;
        }
        break;
      case 'app':
        if (isRecord(v)) {
          mc.app = v;
        } else {
          extra[k] = v;
        }
        break;
      case 'device':
        if (isRecord(v)) {
          mc.device = v;
        } else {
          extra[k] = v;
        }
        break;
      case 'os':
        if (isRecord(v)) {
          mc.os = v;
        } else {
          extra[k] = v;
        }
        break;
      case 'network':
        if (isRecord(v)) {
          mc.network = v;
        } else {
          extra[k] = v;
        }
        break;
      case 'screen':
        if (isRecord(v)) {
          mc.screen = v;
        } else {
          extra[k] = v;
        }
        break;
      default:
        extra[k] = v;
    }
  }

  if (Object.keys(extra).length > 0) {
    mc.extra = extra;
  }

  return mc;
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}
