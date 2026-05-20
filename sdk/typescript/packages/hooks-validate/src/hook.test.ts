import { describe, it, expect } from 'vitest';
import { ValidationHook, ValidationError } from './hook';
import type { EventSchema } from './hook';
import type { HookContext } from '@event-spec/api';

function makeHC(eventName: string, properties: Record<string, unknown> = {}): HookContext {
  return {
    operation: 'track',
    eventName,
    context: {},
    message: { eventName, properties, context: {} },
  };
}

const schema: EventSchema = {
  eventName: 'ButtonClicked',
  properties: {
    label: { type: 'string', required: true },
    count: { type: 'integer', required: false },
  },
};

describe('ValidationHook', () => {
  it('passes through events with no registered schema', async () => {
    const hook = new ValidationHook(() => undefined);
    const result = await hook.before(makeHC('UnknownEvent', { x: 1 }));
    expect(result).toBeNull();
  });

  it('passes events with valid required properties', async () => {
    const hook = new ValidationHook((name) => (name === 'ButtonClicked' ? schema : undefined));
    const result = await hook.before(makeHC('ButtonClicked', { label: 'signup' }));
    expect(result).toBeNull();
  });

  it('passes when hc.message has no properties field', async () => {
    const hook = new ValidationHook((name) => (name === 'ButtonClicked' ? schema : undefined));
    const hc: HookContext = { operation: 'track', eventName: 'ButtonClicked', context: {} };
    const result = await hook.before(hc);
    expect(result).toBeNull();
  });

  it('throws ValidationError for missing required property', async () => {
    const hook = new ValidationHook((name) => (name === 'ButtonClicked' ? schema : undefined));
    await expect(hook.before(makeHC('ButtonClicked', { count: 1 }))).rejects.toThrow(
      ValidationError,
    );
    await expect(hook.before(makeHC('ButtonClicked', { count: 1 }))).rejects.toThrow('label');
  });

  it('throws ValidationError for wrong property type', async () => {
    const hook = new ValidationHook((name) => (name === 'ButtonClicked' ? schema : undefined));
    await expect(hook.before(makeHC('ButtonClicked', { label: 123 }))).rejects.toThrow(
      ValidationError,
    );
  });

  it('ValidationError carries eventName and violations', async () => {
    const hook = new ValidationHook((name) => (name === 'ButtonClicked' ? schema : undefined));
    let caught: unknown;
    try {
      await hook.before(makeHC('ButtonClicked', {}));
    } catch (err) {
      caught = err;
    }
    expect(caught).toBeInstanceOf(ValidationError);
    const ve = caught as ValidationError;
    expect(ve.eventName).toBe('ButtonClicked');
    expect(ve.violations.length).toBeGreaterThan(0);
  });

  it('allows extra properties (additionalProperties: true)', async () => {
    const hook = new ValidationHook((name) => (name === 'ButtonClicked' ? schema : undefined));
    const result = await hook.before(
      makeHC('ButtonClicked', { label: 'ok', extra_field: 'anything' }),
    );
    expect(result).toBeNull();
  });

  it('validates enum constraint', async () => {
    const enumSchema: EventSchema = {
      eventName: 'Signup',
      properties: {
        plan: { type: 'string', required: true, enum: ['free', 'pro', 'enterprise'] },
      },
    };
    const hook = new ValidationHook(() => enumSchema);
    await expect(hook.before(makeHC('Signup', { plan: 'invalid' }))).rejects.toThrow(
      ValidationError,
    );
    const result = await hook.before(makeHC('Signup', { plan: 'pro' }));
    expect(result).toBeNull();
  });

  it('validates minimum constraint', async () => {
    const minSchema: EventSchema = {
      eventName: 'Purchase',
      properties: {
        amount: { type: 'number', required: true, minimum: 0 },
      },
    };
    const hook = new ValidationHook(() => minSchema);
    await expect(hook.before(makeHC('Purchase', { amount: -1 }))).rejects.toThrow(ValidationError);
    const result = await hook.before(makeHC('Purchase', { amount: 0 }));
    expect(result).toBeNull();
  });
});
