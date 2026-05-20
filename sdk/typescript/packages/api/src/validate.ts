import Ajv from 'ajv';
import { UnimplementedHook } from './hooks';
import type { HookContext, HookHints, EventEnvelope } from './hooks';

export type PropertyType = 'string' | 'number' | 'integer' | 'boolean' | 'object' | 'array';

export type EventStatus = 'active' | 'deprecated' | 'deleted' | 'draft';

export interface PropertySchema {
  type: PropertyType;
  required?: boolean;
  description?: string;
  enum?: string[];
  pattern?: string;
  minimum?: number;
  maximum?: number;
  default?: unknown;
}

export interface EventSchema {
  eventName: string;
  status?: EventStatus;
  properties: Record<string, PropertySchema>;
}

export type SchemaLookup = (eventName: string) => EventSchema | undefined;

export interface ValidationViolation {
  field: string;
  message: string;
}

export interface Logger {
  warn(message: string, context?: Record<string, unknown>): void;
}

export class ValidationError extends Error {
  constructor(
    public readonly eventName: string,
    public readonly violations: ValidationViolation[],
  ) {
    const msgs = violations.map((v) => (v.field ? `${v.field}: ${v.message}` : v.message));
    super(`event "${eventName}" failed schema validation: ${msgs.join('; ')}`);
    this.name = 'ValidationError';
  }
}

// DeletedEventError is thrown by before() when the event's spec status is deleted.
// Dispatch is blocked entirely to prevent silent data loss when a retired event
// is still being called at a call site.
export class DeletedEventError extends Error {
  constructor(public readonly eventName: string) {
    super(`event "${eventName}" is deleted and cannot be dispatched`);
    this.name = 'DeletedEventError';
  }
}

// ValidationHook validates event properties against a registered JSON Schema in the Before stage.
// It also gates dispatch on the event's lifecycle status: deleted events are rejected with
// DeletedEventError; deprecated events emit a structured warning log but proceed normally.
// Events with no registered schema pass through unchanged, so this hook works alongside
// codegen compile-time safety rather than replacing it.
export class ValidationHook extends UnimplementedHook {
  private readonly ajv = new Ajv({ allErrors: true });
  // Cache compiled validators per event name to avoid recompiling on every call.
  private readonly cache = new Map<string, ReturnType<Ajv['compile']>>();

  // logger is optional; if not provided, deprecation warnings are silently skipped.
  constructor(
    private readonly lookup: SchemaLookup,
    private readonly logger?: Logger,
  ) {
    super();
  }

  override async before(hc: HookContext, _hints?: HookHints): Promise<EventEnvelope | null> {
    const schema = this.lookup(hc.eventName);
    if (!schema) return null;

    if (schema.status === 'deleted') {
      throw new DeletedEventError(hc.eventName);
    }

    if (schema.status === 'deprecated' && this.logger) {
      this.logger.warn('event is deprecated — update call sites', { event: hc.eventName });
    }

    const properties = extractProperties(hc);
    if (properties === null) return null;

    const validate = this.getValidator(hc.eventName, schema);
    if (validate(properties)) return null;

    const violations = (validate.errors ?? []).map((err) => ({
      field: err.instancePath.replace(/^\//, '') || '',
      message: err.message ?? 'validation failed',
    }));
    throw new ValidationError(hc.eventName, violations);
  }

  private getValidator(eventName: string, schema: EventSchema): ReturnType<Ajv['compile']> {
    const cached = this.cache.get(eventName);
    if (cached) return cached;
    const jsonSchema = buildJSONSchema(schema);
    const validate = this.ajv.compile(jsonSchema);
    this.cache.set(eventName, validate);
    return validate;
  }
}

function extractProperties(hc: HookContext): Record<string, unknown> | null {
  const msg = hc.message;
  if (!msg || typeof msg !== 'object') return null;
  const record = msg as Record<string, unknown>;
  if (
    'properties' in record &&
    record['properties'] !== null &&
    typeof record['properties'] === 'object' &&
    !Array.isArray(record['properties'])
  ) {
    return record['properties'] as Record<string, unknown>;
  }
  return null;
}

function buildJSONSchema(schema: EventSchema): object {
  const properties: Record<string, unknown> = {};
  const required: string[] = [];

  for (const [name, prop] of Object.entries(schema.properties)) {
    const propSchema: Record<string, unknown> = { type: prop.type };
    if (prop.description) propSchema['description'] = prop.description;
    if (prop.enum?.length) propSchema['enum'] = prop.enum;
    if (prop.pattern) propSchema['pattern'] = prop.pattern;
    if (prop.minimum !== undefined) propSchema['minimum'] = prop.minimum;
    if (prop.maximum !== undefined) propSchema['maximum'] = prop.maximum;
    if (prop.default !== undefined) propSchema['default'] = prop.default;
    properties[name] = propSchema;
    if (prop.required) required.push(name);
  }

  const jsonSchema: Record<string, unknown> = {
    $schema: 'http://json-schema.org/draft-07/schema#',
    title: schema.eventName,
    type: 'object',
    properties,
    additionalProperties: true,
  };
  if (required.length > 0) jsonSchema['required'] = required;
  return jsonSchema;
}
