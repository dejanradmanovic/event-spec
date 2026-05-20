import { describe, it, expect, beforeEach } from 'vitest';
import { Client, setGlobalContext, setGlobalProvider, addGlobalHooks } from './client';
import type { DispatchResult, Event, TrackOptions } from './client';
import { UnimplementedHook } from './hooks';
import type { Hook, HookContext, HookHints, EventEnvelope } from './hooks';
import type {
  Provider,
  ProviderMetadata,
  TrackMessage,
  IdentifyMessage,
  GroupMessage,
  PageMessage,
  AliasMessage,
} from './provider';

// CaptureProvider records all messages for assertion.
class CaptureProvider implements Provider {
  readonly tracked: TrackMessage[] = [];
  readonly identified: IdentifyMessage[] = [];
  readonly grouped: GroupMessage[] = [];
  readonly paged: PageMessage[] = [];
  readonly aliased: AliasMessage[] = [];
  private readonly _name: string;

  constructor(name = 'capture') {
    this._name = name;
  }

  metadata(): ProviderMetadata {
    return {
      name: this._name,
      version: '0.1.0',
      capabilities: { track: true, identify: true, group: true, page: true, alias: true },
    };
  }

  hooks(): Hook[] {
    return [];
  }
  async track(msg: TrackMessage): Promise<void> {
    this.tracked.push(msg);
  }
  async identify(msg: IdentifyMessage): Promise<void> {
    this.identified.push(msg);
  }
  async group(msg: GroupMessage): Promise<void> {
    this.grouped.push(msg);
  }
  async page(msg: PageMessage): Promise<void> {
    this.paged.push(msg);
  }
  async alias(msg: AliasMessage): Promise<void> {
    this.aliased.push(msg);
  }
  async flush(): Promise<void> {}
  async shutdown(): Promise<void> {}
}

// FailingProvider always rejects track calls.
class FailingProvider extends CaptureProvider {
  constructor() {
    super('failing');
  }
  override async track(): Promise<void> {
    throw new Error('provider down');
  }
}

beforeEach(() => {
  // Reset global state between tests
  setGlobalContext({});
  setGlobalProvider();
  // Clear global hooks by reinitialising them — indirect approach via a new provider
  // (global hooks accumulate; tests that add global hooks should use a dedicated Client)
});

describe('Client.track', () => {
  it('dispatches track event to configured providers', async () => {
    const p = new CaptureProvider();
    const client = new Client({ providers: [p] });
    const event: Event = { name: 'Button Clicked', properties: { label: 'signup' } };
    await client.track(event);

    expect(p.tracked).toHaveLength(1);
    expect(p.tracked[0].eventName).toBe('Button Clicked');
    expect(p.tracked[0].properties['label']).toBe('signup');
  });

  it('dispatches to multiple providers concurrently', async () => {
    const p1 = new CaptureProvider('p1');
    const p2 = new CaptureProvider('p2');
    const client = new Client({ providers: [p1, p2] });
    await client.track({ name: 'Multi Provider' });

    expect(p1.tracked).toHaveLength(1);
    expect(p2.tracked).toHaveLength(1);
  });

  it('generates a unique messageId per call', async () => {
    const p = new CaptureProvider();
    const client = new Client({ providers: [p] });
    await client.track({ name: 'E1' });
    await client.track({ name: 'E2' });

    expect(p.tracked[0].messageId).not.toBe(p.tracked[1].messageId);
  });
});

describe('Client.trackDetailed', () => {
  it('returns success result for working provider', async () => {
    const p = new CaptureProvider();
    const client = new Client({ providers: [p] });
    const result: DispatchResult = await client.trackDetailed({ name: 'Ev' });

    expect(result.success).toHaveLength(1);
    expect(result.success[0].providerName).toBe('capture');
    expect(result.failed).toHaveLength(0);
    expect(result.partialSuccess).toBe(true);
  });

  it('returns failed result when provider throws', async () => {
    const failing = new FailingProvider();
    const client = new Client({ providers: [failing] });
    const result = await client.trackDetailed({ name: 'Ev' });

    expect(result.failed).toHaveLength(1);
    expect(result.failed[0].error?.message).toBe('provider down');
    expect(result.partialSuccess).toBe(false);
  });

  it('partial success when one of two providers fails', async () => {
    const good = new CaptureProvider('good');
    const bad = new FailingProvider();
    const client = new Client({ providers: [good, bad] });
    const result = await client.trackDetailed({ name: 'Ev' });

    expect(result.partialSuccess).toBe(true);
    expect(result.success).toHaveLength(1);
    expect(result.failed).toHaveLength(1);
  });

  it('returns empty result when no providers are configured', async () => {
    const client = new Client();
    const result = await client.trackDetailed({ name: 'Ev' });
    expect(result.success).toHaveLength(0);
    expect(result.failed).toHaveLength(0);
    expect(result.partialSuccess).toBe(false);
  });
});

describe('Context merge (4-level chain)', () => {
  it('global context is level 1 (lowest priority)', async () => {
    setGlobalContext({ userId: 'global-user' });
    const p = new CaptureProvider();
    const client = new Client({ providers: [p] });
    await client.track({ name: 'Ev' });
    expect(p.tracked[0].userId).toBe('global-user');
  });

  it('client context (level 3) overrides global (level 1)', async () => {
    setGlobalContext({ userId: 'global-user' });
    const p = new CaptureProvider();
    const client = new Client({ providers: [p], context: { userId: 'client-user' } });
    await client.track({ name: 'Ev' });
    expect(p.tracked[0].userId).toBe('client-user');
  });

  it('withTransaction (level 2) overrides global but not client', async () => {
    setGlobalContext({ userId: 'global-user' });
    const p = new CaptureProvider();
    const base = new Client({ providers: [p], context: { userId: 'client-user' } });
    const scoped = base.withTransaction({ userId: 'tx-user' });
    await scoped.track({ name: 'Ev' });
    // client (level 3) > transaction (level 2)
    expect(p.tracked[0].userId).toBe('client-user');
  });

  it('invocation override (level 4) wins over all', async () => {
    setGlobalContext({ userId: 'global-user' });
    const p = new CaptureProvider();
    const client = new Client({ providers: [p], context: { userId: 'client-user' } });
    const opts: TrackOptions = { contextOverride: { userId: 'invocation-user' } };
    await client.track({ name: 'Ev' }, opts);
    expect(p.tracked[0].userId).toBe('invocation-user');
  });

  it('withTransaction binds transaction context at level 2', async () => {
    const p = new CaptureProvider();
    const base = new Client({ providers: [p] });
    const scoped = base.withTransaction({ userId: 'tx-user', anonymousId: 'anon-123' });
    await scoped.track({ name: 'Ev' });
    expect(p.tracked[0].userId).toBe('tx-user');
    expect(p.tracked[0].anonymousId).toBe('anon-123');
  });
});

describe('Hook chain integration', () => {
  it('before hook can mutate the event name', async () => {
    const p = new CaptureProvider();

    class RenameHook extends UnimplementedHook {
      override async before(_hc: HookContext): Promise<EventEnvelope | null> {
        return { eventName: 'Renamed', properties: {}, context: {} };
      }
    }

    const client = new Client({ providers: [p], hooks: [new RenameHook()] });
    await client.track({ name: 'Original' });
    expect(p.tracked[0].eventName).toBe('Renamed');
  });

  it('before hook cancellation prevents dispatch', async () => {
    const p = new CaptureProvider();

    class CancelHook extends UnimplementedHook {
      override async before(): Promise<EventEnvelope | null> {
        throw new Error('event cancelled by hook');
      }
    }

    const client = new Client({ providers: [p], hooks: [new CancelHook()] });
    await expect(client.track({ name: 'Ev' })).rejects.toThrow('event cancelled by hook');
    expect(p.tracked).toHaveLength(0);
  });

  it('after/error/finally fire per provider in reverse hook order', async () => {
    const p = new CaptureProvider();
    const log: string[] = [];

    class LogHook extends UnimplementedHook {
      constructor(private readonly id: string) {
        super();
      }
      override async after(_hc: HookContext, _result: unknown, _hints?: HookHints): Promise<void> {
        log.push(`${this.id}:after`);
      }
      override finally(): void {
        log.push(`${this.id}:finally`);
      }
    }

    const client = new Client({ providers: [p], hooks: [new LogHook('h1'), new LogHook('h2')] });
    await client.track({ name: 'Ev' });
    expect(log).toEqual(['h2:after', 'h1:after', 'h2:finally', 'h1:finally']);
  });

  it('provider-level hooks are appended after client hooks', async () => {
    const log: string[] = [];

    class LogHook extends UnimplementedHook {
      constructor(private readonly id: string) {
        super();
      }
      override async before(): Promise<EventEnvelope | null> {
        log.push(`${this.id}:before`);
        return null;
      }
    }

    class ProviderWithHook extends CaptureProvider {
      override hooks(): Hook[] {
        return [new LogHook('prov')];
      }
    }

    const p = new ProviderWithHook('p1');
    const client = new Client({ providers: [p], hooks: [new LogHook('client')] });
    await client.track({ name: 'Ev' });
    expect(log).toEqual(['client:before', 'prov:before']);
  });
});

describe('identify / group / page / alias', () => {
  it('identify dispatches to provider with userId and traits', async () => {
    const p = new CaptureProvider();
    const client = new Client({ providers: [p] });
    await client.identify('user-1', { email: 'a@b.com' });
    expect(p.identified).toHaveLength(1);
    expect(p.identified[0].userId).toBe('user-1');
    expect(p.identified[0].traits['email']).toBe('a@b.com');
  });

  it('group dispatches to provider with groupId and traits', async () => {
    const p = new CaptureProvider();
    const client = new Client({ providers: [p] });
    await client.group('org-1', { plan: 'pro' });
    expect(p.grouped).toHaveLength(1);
    expect(p.grouped[0].groupId).toBe('org-1');
    expect(p.grouped[0].traits['plan']).toBe('pro');
  });

  it('page dispatches to provider with name and properties', async () => {
    const p = new CaptureProvider();
    const client = new Client({ providers: [p] });
    await client.page('/home', { referrer: 'google' });
    expect(p.paged).toHaveLength(1);
    expect(p.paged[0].name).toBe('/home');
    expect(p.paged[0].properties['referrer']).toBe('google');
  });

  it('alias dispatches to provider with userId and previousId', async () => {
    const p = new CaptureProvider();
    const client = new Client({ providers: [p] });
    await client.alias('new-user', 'anon-prev');
    expect(p.aliased).toHaveLength(1);
    expect(p.aliased[0].userId).toBe('new-user');
    expect(p.aliased[0].previousId).toBe('anon-prev');
  });
});

describe('Client.setContext', () => {
  it('updates client context after construction', async () => {
    const p = new CaptureProvider();
    const client = new Client({ providers: [p] });
    client.setContext({ userId: 'updated' });
    await client.track({ name: 'Ev' });
    expect(p.tracked[0].userId).toBe('updated');
  });
});

describe('global API', () => {
  it('addGlobalHooks adds hooks that run for all clients', async () => {
    const log: string[] = [];

    class GlobalHook extends UnimplementedHook {
      override async before(): Promise<EventEnvelope | null> {
        log.push('global:before');
        return null;
      }
    }

    addGlobalHooks(new GlobalHook());
    const p = new CaptureProvider();
    const client = new Client({ providers: [p] });
    await client.track({ name: 'Ev' });
    expect(log).toContain('global:before');
  });
});
