import { describe, it, expect } from 'vitest';
import { SamplingHook, SampledError } from './hook';
import type { SamplingPolicy } from './hook';
import type { HookContext } from '@event-spec/api';

function makeHC(eventName: string, userId = ''): HookContext {
  return { operation: 'track', eventName, context: { userId } };
}

describe('SamplingHook', () => {
  it('passes through events with no registered policy', async () => {
    const hook = new SamplingHook(() => undefined);
    const result = await hook.before(makeHC('Ev'));
    expect(result).toBeNull();
  });

  it('passes through events with strategy "none"', async () => {
    const hook = new SamplingHook((): SamplingPolicy => ({ strategy: 'none', rate: 0.5 }));
    const result = await hook.before(makeHC('Ev'));
    expect(result).toBeNull();
  });

  it('random strategy with rate 0 always drops', async () => {
    const hook = new SamplingHook((): SamplingPolicy => ({ strategy: 'random', rate: 0 }));
    await expect(hook.before(makeHC('Ev'))).rejects.toThrow(SampledError);
  });

  it('random strategy with rate 1 always keeps', async () => {
    const hook = new SamplingHook((): SamplingPolicy => ({ strategy: 'random', rate: 1 }));
    const result = await hook.before(makeHC('Ev'));
    expect(result).toBeNull();
  });

  it('user_id_hash is deterministic for the same userId', async () => {
    const hook = new SamplingHook((): SamplingPolicy => ({ strategy: 'user_id_hash', rate: 0.5 }));
    const hc = makeHC('Ev', 'user-abc-123');

    const decisions: boolean[] = [];
    for (let i = 0; i < 5; i++) {
      let kept = true;
      try {
        await hook.before(hc);
      } catch {
        kept = false;
      }
      decisions.push(kept);
    }
    // All 5 runs must agree — deterministic hashing means the same user always gets the same decision
    expect(new Set(decisions).size).toBe(1);
  });

  it('user_id_hash with rate 0 always drops (norm >= 0)', async () => {
    const hook = new SamplingHook((): SamplingPolicy => ({ strategy: 'user_id_hash', rate: 0 }));
    await expect(hook.before(makeHC('Ev', 'any-user'))).rejects.toThrow(SampledError);
  });

  it('user_id_hash with rate 1 always keeps (norm < 1)', async () => {
    const hook = new SamplingHook((): SamplingPolicy => ({ strategy: 'user_id_hash', rate: 1 }));
    const result = await hook.before(makeHC('Ev', 'any-user'));
    expect(result).toBeNull();
  });

  it('SampledError has the correct message', async () => {
    const hook = new SamplingHook((): SamplingPolicy => ({ strategy: 'random', rate: 0 }));
    let err: unknown;
    try {
      await hook.before(makeHC('Ev'));
    } catch (e) {
      err = e;
    }
    expect(err).toBeInstanceOf(SampledError);
    expect((err as SampledError).message).toBe('event dropped by sampling policy');
  });
});
