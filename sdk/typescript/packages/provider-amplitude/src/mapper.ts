import type {
  TrackMessage,
  IdentifyMessage,
  GroupMessage,
  AliasMessage,
  MessageContext,
} from '@dejanradmanovic/event-spec-api';

const MAX_STRING_CHARS = 1024;

// AmplitudeEvent is the Amplitude HTTP batch event schema.
export interface AmplitudeEvent {
  user_id?: string;
  device_id?: string;
  event_type: string;
  time: number;
  event_properties?: Record<string, unknown>;
  user_properties?: Record<string, unknown>;
  group_type?: string;
  group_value?: string;
  group_properties?: Record<string, unknown>;
  insert_id?: string;
  ip?: string;
  language?: string;
  os_name?: string;
  platform?: string;
}

export interface AmplitudeBatchRequest {
  api_key: string;
  events: AmplitudeEvent[];
}

export function mapTrackMessage(msg: TrackMessage): AmplitudeEvent {
  const ts = msg.timestamp ?? new Date();
  const ev: AmplitudeEvent = {
    user_id: msg.userId || undefined,
    device_id: msg.anonymousId || undefined,
    event_type: msg.eventName,
    time: ts.getTime(),
    event_properties: coerceProperties(msg.properties),
    insert_id: msg.messageId,
  };
  applyMessageContext(ev, msg.context);
  return ev;
}

export function mapIdentifyMessage(msg: IdentifyMessage): AmplitudeEvent {
  const ts = msg.timestamp ?? new Date();
  const ev: AmplitudeEvent = {
    user_id: msg.userId || undefined,
    device_id: msg.anonymousId || undefined,
    event_type: '$identify',
    time: ts.getTime(),
    user_properties: { $set: coerceProperties(msg.traits) },
    insert_id: msg.messageId,
  };
  applyMessageContext(ev, msg.context);
  return ev;
}

export function mapGroupMessage(msg: GroupMessage): AmplitudeEvent {
  const ts = msg.timestamp ?? new Date();
  const ev: AmplitudeEvent = {
    user_id: msg.userId || undefined,
    device_id: msg.anonymousId || undefined,
    event_type: '$groupidentify',
    time: ts.getTime(),
    group_type: 'group',
    group_value: msg.groupId,
    group_properties: { $set: coerceProperties(msg.traits) },
    insert_id: msg.messageId,
  };
  applyMessageContext(ev, msg.context);
  return ev;
}

// mapAliasMessage maps an alias call to an Amplitude $identify event that links
// previousId (device_id) to the new userId, merging the two identities.
export function mapAliasMessage(msg: AliasMessage): AmplitudeEvent {
  const ts = msg.timestamp ?? new Date();
  const ev: AmplitudeEvent = {
    user_id: msg.userId,
    device_id: msg.previousId,
    event_type: '$identify',
    time: ts.getTime(),
    insert_id: msg.messageId,
  };
  applyMessageContext(ev, msg.context);
  return ev;
}

function applyMessageContext(ev: AmplitudeEvent, mc: MessageContext): void {
  if (mc.ipAddress) ev.ip = mc.ipAddress;
  if (mc.locale) ev.language = mc.locale;
  const osName = mc.os?.['name'];
  if (typeof osName === 'string' && osName) ev.os_name = osName;
  const platform = mc.app?.['platform'];
  if (typeof platform === 'string' && platform) ev.platform = platform;

  // Merge extra context attributes into event_properties so session_id, platform, etc. reach Amplitude.
  if (mc.extra && Object.keys(mc.extra).length > 0) {
    if (!ev.event_properties) ev.event_properties = {};
    for (const [k, v] of Object.entries(mc.extra)) {
      if (!(k in ev.event_properties)) {
        ev.event_properties[k] = coerceValue(v);
      }
    }
  }
}

export function coerceProperties(
  props: Record<string, unknown> | undefined,
): Record<string, unknown> | undefined {
  if (!props) return undefined;
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(props)) {
    out[k] = coerceValue(v);
  }
  return out;
}

function coerceValue(v: unknown): unknown {
  if (typeof v === 'string') return truncateString(v);
  if (Array.isArray(v)) return v.map(coerceValue);
  if (v !== null && typeof v === 'object') return coerceProperties(v as Record<string, unknown>);
  return v;
}

// truncateString truncates s to at most MAX_STRING_CHARS Unicode code points.
export function truncateString(s: string): string {
  const codePoints = [...s]; // spread iterates Unicode code points, not UTF-16 units
  if (codePoints.length <= MAX_STRING_CHARS) return s;
  return codePoints.slice(0, MAX_STRING_CHARS).join('');
}
