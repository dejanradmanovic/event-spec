import { describe, it, expect, vi } from 'vitest';
import { ValidationHook, ValidationError, DeletedEventError } from './validate';
import type { EventSchema, Logger } from './validate';
import type { HookContext } from './hooks';

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

  describe('status gating', () => {
    const baseSchema: EventSchema = {
      eventName: 'ButtonClicked',
      properties: { label: { type: 'string', required: true } },
    };

    it('throws DeletedEventError for deleted events', async () => {
      const deletedSchema: EventSchema = { ...baseSchema, status: 'deleted' };
      const hook = new ValidationHook(() => deletedSchema);
      await expect(hook.before(makeHC('ButtonClicked', { label: 'ok' }))).rejects.toThrow(
        DeletedEventError,
      );
    });

    it('DeletedEventError carries eventName', async () => {
      const deletedSchema: EventSchema = { ...baseSchema, status: 'deleted' };
      const hook = new ValidationHook(() => deletedSchema);
      let caught: unknown;
      try {
        await hook.before(makeHC('ButtonClicked', {}));
      } catch (err) {
        caught = err;
      }
      expect(caught).toBeInstanceOf(DeletedEventError);
      expect((caught as DeletedEventError).eventName).toBe('ButtonClicked');
    });

    it('emits a deprecation warning for deprecated events and proceeds', async () => {
      const deprecatedSchema: EventSchema = { ...baseSchema, status: 'deprecated' };
      const logger: Logger = { warn: vi.fn() };
      const hook = new ValidationHook(() => deprecatedSchema, logger);
      const result = await hook.before(makeHC('ButtonClicked', { label: 'ok' }));
      expect(result).toBeNull();
      expect(logger.warn).toHaveBeenCalledOnce();
      expect(logger.warn).toHaveBeenCalledWith(
        expect.stringContaining('deprecated'),
        expect.objectContaining({ event: 'ButtonClicked' }),
      );
    });

    it('skips deprecation warning silently when no logger is provided', async () => {
      const deprecatedSchema: EventSchema = { ...baseSchema, status: 'deprecated' };
      const hook = new ValidationHook(() => deprecatedSchema);
      const result = await hook.before(makeHC('ButtonClicked', { label: 'ok' }));
      expect(result).toBeNull();
    });

    it('passes through draft events without blocking', async () => {
      const draftSchema: EventSchema = { ...baseSchema, status: 'draft' };
      const hook = new ValidationHook(() => draftSchema);
      const result = await hook.before(makeHC('ButtonClicked', { label: 'ok' }));
      expect(result).toBeNull();
    });

    it('passes through active events normally', async () => {
      const activeSchema: EventSchema = { ...baseSchema, status: 'active' };
      const hook = new ValidationHook(() => activeSchema);
      const result = await hook.before(makeHC('ButtonClicked', { label: 'ok' }));
      expect(result).toBeNull();
    });
  });
});
