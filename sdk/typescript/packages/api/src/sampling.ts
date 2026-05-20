import { UnimplementedHook } from './hooks';
import type { HookContext, HookHints, EventEnvelope } from './hooks';

export type SamplingStrategy = 'user_id_hash' | 'random' | 'none';

export interface SamplingPolicy {
  strategy: SamplingStrategy;
  rate: number;
}

export type SamplingLookup = (eventName: string) => SamplingPolicy | undefined;

// SampledError is thrown by SamplingHook.before when an event is dropped by the sampling policy.
export class SampledError extends Error {
  constructor() {
    super('event dropped by sampling policy');
    this.name = 'SampledError';
  }
}

// SamplingHook applies the sampling policy declared for an event during the Before stage.
// Sampled-out events throw SampledError; events without a policy pass through unchanged.
export class SamplingHook extends UnimplementedHook {
  constructor(private readonly lookup: SamplingLookup) {
    super();
  }

  override async before(hc: HookContext, _hints?: HookHints): Promise<EventEnvelope | null> {
    const policy = this.lookup(hc.eventName);
    if (!policy || !policy.strategy || policy.strategy === 'none') return null;

    let keep: boolean;
    switch (policy.strategy) {
      case 'user_id_hash':
        keep = hashKeep(hc.context.userId ?? '', policy.rate);
        break;
      case 'random':
        keep = Math.random() < policy.rate;
        break;
      default:
        return null;
    }

    if (!keep) throw new SampledError();
    return null;
  }
}

// fnv1a32 computes the FNV-1a 32-bit hash of s, operating on UTF-8 bytes.
// Matches Go's hash/fnv New32a implementation exactly.
function fnv1a32(s: string): number {
  const bytes = new TextEncoder().encode(s);
  let h = 2166136261; // FNV offset basis (32-bit)
  for (const byte of bytes) {
    h ^= byte;
    h = Math.imul(h, 16777619) >>> 0; // FNV prime, keep 32-bit unsigned
  }
  return h >>> 0;
}

// hashKeep returns true when userId hashes into the kept fraction [0, rate).
// The decision is deterministic: the same userId always produces the same outcome.
function hashKeep(userId: string, rate: number): boolean {
  const norm = fnv1a32(userId) / 0x100000000; // normalise to [0, 1)
  return norm < rate;
}
