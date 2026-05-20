import { describe, it, expect } from 'vitest';
import { HookChain, UnimplementedHook } from './hooks';
import type { Hook, HookContext, HookHints, HookResult, EventEnvelope } from './hooks';

function baseHC(): HookContext {
  return { operation: 'track', eventName: 'Test Event', context: {} };
}

// recorder logs each invoked stage as "name:stage" into a shared array.
class Recorder extends UnimplementedHook {
  constructor(
    private readonly name: string,
    private readonly log: string[],
  ) {
    super();
  }

  override async before(_hc: HookContext, _hints?: HookHints): Promise<EventEnvelope | null> {
    this.log.push(`${this.name}:before`);
    return null;
  }

  override async after(_hc: HookContext, _result: HookResult, _hints?: HookHints): Promise<void> {
    this.log.push(`${this.name}:after`);
  }

  override error(_hc: HookContext, _err: Error, _hints?: HookHints): void {
    this.log.push(`${this.name}:error`);
  }

  override finally(_hc: HookContext, _result: HookResult, _hints?: HookHints): void {
    this.log.push(`${this.name}:finally`);
  }
}

// CancelHook cancels the before chain by throwing.
class CancelHook extends UnimplementedHook {
  constructor(private readonly err: Error) {
    super();
  }

  override async before(): Promise<EventEnvelope | null> {
    throw this.err;
  }
}

// RenameHook replaces EventName via the returned EventEnvelope.
class RenameHook extends UnimplementedHook {
  constructor(private readonly newName: string) {
    super();
  }

  override async before(hc: HookContext): Promise<EventEnvelope | null> {
    return { eventName: this.newName, properties: {}, context: hc.context };
  }
}

describe('HookChain.before', () => {
  it('runs in forward order', async () => {
    const log: string[] = [];
    const chain = new HookChain([
      new Recorder('h1', log),
      new Recorder('h2', log),
      new Recorder('h3', log),
    ]);
    await chain.before(baseHC());
    expect(log).toEqual(['h1:before', 'h2:before', 'h3:before']);
  });

  it('cancellation stops subsequent hooks', async () => {
    const log: string[] = [];
    const sentinel = new Error('consent denied');
    const chain = new HookChain([new CancelHook(sentinel), new Recorder('never', log)]);
    await expect(chain.before(baseHC())).rejects.toThrow('consent denied');
    expect(log).toHaveLength(0);
  });

  it('mutation propagates eventName to subsequent hooks', async () => {
    let seen = '';
    const observer: Hook = {
      async before(hc) {
        seen = hc.eventName;
        return null;
      },
      async after() {},
      error() {},
      finally() {},
    };
    const chain = new HookChain([new RenameHook('Renamed'), observer]);
    await chain.before(baseHC());
    expect(seen).toBe('Renamed');
  });

  it('returns the final EventEnvelope from the last mutating hook', async () => {
    const chain = new HookChain([new RenameHook('First'), new RenameHook('Second')]);
    const env = await chain.before(baseHC());
    expect(env?.eventName).toBe('Second');
  });

  it('returns null when no hook mutates the event', async () => {
    const log: string[] = [];
    const chain = new HookChain([new Recorder('h1', log)]);
    const env = await chain.before(baseHC());
    expect(env).toBeNull();
  });
});

describe('HookChain.after', () => {
  it('runs in reverse order', async () => {
    const log: string[] = [];
    const chain = new HookChain([
      new Recorder('h1', log),
      new Recorder('h2', log),
      new Recorder('h3', log),
    ]);
    await chain.after(baseHC(), { delivered: true, dropped: false });
    expect(log).toEqual(['h3:after', 'h2:after', 'h1:after']);
  });
});

describe('HookChain.error', () => {
  it('runs in reverse order', () => {
    const log: string[] = [];
    const chain = new HookChain([new Recorder('h1', log), new Recorder('h2', log)]);
    chain.error(baseHC(), new Error('provider down'));
    expect(log).toEqual(['h2:error', 'h1:error']);
  });
});

describe('HookChain.finally', () => {
  it('runs in reverse order', () => {
    const log: string[] = [];
    const chain = new HookChain([new Recorder('h1', log), new Recorder('h2', log)]);
    chain.finally(baseHC(), { delivered: true, dropped: false });
    expect(log).toEqual(['h2:finally', 'h1:finally']);
  });
});

describe('UnimplementedHook', () => {
  it('before returns null', async () => {
    const h = new UnimplementedHook();
    expect(await h.before(baseHC())).toBeNull();
  });

  it('after resolves without error', async () => {
    const h = new UnimplementedHook();
    await expect(h.after(baseHC(), { delivered: true, dropped: false })).resolves.toBeUndefined();
  });
});
