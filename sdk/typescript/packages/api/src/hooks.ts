import type { AnalyticsContext } from './context';

export type Operation = 'track' | 'identify' | 'group' | 'page' | 'alias';

export interface HookContext {
  operation: Operation;
  eventName: string;
  context: AnalyticsContext;
  message?: unknown;
  provider?: string;
}

// EventEnvelope is the mutable event representation passed through Before hooks.
// Return a non-null EventEnvelope from before() to replace the event for subsequent hooks and providers.
export interface EventEnvelope {
  eventName: string;
  properties: Record<string, unknown>;
  context: AnalyticsContext;
  metadata?: Record<string, unknown>;
}

export type HookHints = Record<string, unknown>;

export interface HookResult {
  delivered: boolean;
  dropped: boolean;
  error?: Error;
}

// Hook is the middleware interface for the analytics event pipeline.
//
// Ordering is governance-first: api-hooks → client-hooks → provider-hooks for before;
// the reverse for after/error/finally.
//
// before runs once, gating all providers. after/error/finally fire once per provider result.
export interface Hook {
  // before runs once before dispatch. Return a non-null EventEnvelope to replace the event
  // for subsequent hooks and providers. Throw to cancel the event entirely.
  before(hc: HookContext, hints?: HookHints): Promise<EventEnvelope | null>;

  // after is called per-provider on success, in reverse hook order.
  after(hc: HookContext, result: HookResult, hints?: HookHints): Promise<void>;

  // error is called per-provider on failure, in reverse hook order. Must not throw.
  error(hc: HookContext, err: Error, hints?: HookHints): void;

  // finally is always called after after or error (defer semantics), in reverse hook order.
  finally(hc: HookContext, result: HookResult, hints?: HookHints): void;
}

// UnimplementedHook provides no-op implementations of all Hook methods.
// Extend this class and override only the stages you need.
export class UnimplementedHook implements Hook {
  async before(_hc: HookContext, _hints?: HookHints): Promise<EventEnvelope | null> {
    return null;
  }

  async after(_hc: HookContext, _result: HookResult, _hints?: HookHints): Promise<void> {}

  error(_hc: HookContext, _err: Error, _hints?: HookHints): void {}

  finally(_hc: HookContext, _result: HookResult, _hints?: HookHints): void {}
}

// HookChain is a slice of hooks that implements the hook chain executor.
//
// before runs in forward (governance-first) order: each non-null EventEnvelope returned by a
// hook replaces the active event for all subsequent hooks. A thrown error cancels the chain.
// after, error, and finally run in reverse order (provider → client → api).
export class HookChain {
  constructor(private readonly hooks: Hook[]) {}

  async before(hc: HookContext, hints?: HookHints): Promise<EventEnvelope | null> {
    let latest: EventEnvelope | null = null;
    const mutableHC = { ...hc };
    for (const h of this.hooks) {
      const result = await h.before(mutableHC, hints);
      if (result !== null) {
        latest = result;
        mutableHC.eventName = result.eventName;
        mutableHC.context = result.context;
      }
    }
    return latest;
  }

  async after(hc: HookContext, result: HookResult, hints?: HookHints): Promise<void> {
    for (let i = this.hooks.length - 1; i >= 0; i--) {
      await this.hooks[i].after(hc, result, hints);
    }
  }

  error(hc: HookContext, err: Error, hints?: HookHints): void {
    for (let i = this.hooks.length - 1; i >= 0; i--) {
      this.hooks[i].error(hc, err, hints);
    }
  }

  finally(hc: HookContext, result: HookResult, hints?: HookHints): void {
    for (let i = this.hooks.length - 1; i >= 0; i--) {
      this.hooks[i].finally(hc, result, hints);
    }
  }
}
