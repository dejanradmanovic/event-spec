import { describe, it, expect } from 'vitest';
import { merge } from './context';
import type { AnalyticsContext } from './context';

describe('merge', () => {
  it('override userId wins over base', () => {
    const result = merge({ userId: 'base' }, { userId: 'override' });
    expect(result.userId).toBe('override');
  });

  it('base userId used when override is empty', () => {
    const result = merge({ userId: 'base' }, {});
    expect(result.userId).toBe('base');
  });

  it('override anonymousId wins over base', () => {
    const result = merge({ anonymousId: 'base-anon' }, { anonymousId: 'override-anon' });
    expect(result.anonymousId).toBe('override-anon');
  });

  it('base anonymousId used when override is empty', () => {
    const result = merge({ anonymousId: 'base-anon' }, {});
    expect(result.anonymousId).toBe('base-anon');
  });

  it('attributes merged key-by-key with override winning', () => {
    const base: AnalyticsContext = { attributes: { a: 1, b: 2 } };
    const override: AnalyticsContext = { attributes: { b: 99, c: 3 } };
    const result = merge(base, override);
    expect(result.attributes).toEqual({ a: 1, b: 99, c: 3 });
  });

  it('base attributes included when override has no attributes', () => {
    const base: AnalyticsContext = { attributes: { x: 'val' } };
    const result = merge(base, {});
    expect(result.attributes).toEqual({ x: 'val' });
  });

  it('override attributes included when base has no attributes', () => {
    const override: AnalyticsContext = { attributes: { y: 'val' } };
    const result = merge({}, override);
    expect(result.attributes).toEqual({ y: 'val' });
  });

  it('no attributes field when neither has attributes', () => {
    const result = merge({}, {});
    expect(result.attributes).toBeUndefined();
  });

  it('4-level chain: each level overrides the previous', () => {
    const global: AnalyticsContext = { userId: 'g', attributes: { platform: 'web' } };
    const transaction: AnalyticsContext = { userId: 'tx', attributes: { session: '123' } };
    const client: AnalyticsContext = { anonymousId: 'anon' };
    const invocation: AnalyticsContext = { userId: 'inv', attributes: { platform: 'mobile' } };

    let result = merge(global, transaction);
    result = merge(result, client);
    result = merge(result, invocation);

    expect(result.userId).toBe('inv');
    expect(result.anonymousId).toBe('anon');
    expect(result.attributes).toEqual({ platform: 'mobile', session: '123' });
  });

  it('empty string override does not overwrite base', () => {
    const result = merge({ userId: 'base' }, { userId: '' });
    expect(result.userId).toBe('base');
  });
});
