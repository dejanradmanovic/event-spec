import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { AmplitudeProvider } from './provider';
import {
  truncateString,
  mapTrackMessage,
  mapIdentifyMessage,
  mapGroupMessage,
  mapAliasMessage,
} from './mapper';
import { ErrUnsupportedOperation } from '@dejanradmanovic/event-spec-api';
import type {
  TrackMessage,
  IdentifyMessage,
  GroupMessage,
  AliasMessage,
} from '@dejanradmanovic/event-spec-api';

const baseTrack: TrackMessage = {
  messageId: 'msg-1',
  timestamp: new Date(1000),
  eventName: 'Button Clicked',
  properties: { label: 'signup' },
  userId: 'u1',
  anonymousId: 'a1',
  context: {},
};

const baseIdentify: IdentifyMessage = {
  messageId: 'msg-2',
  timestamp: new Date(2000),
  userId: 'u1',
  anonymousId: 'a1',
  traits: { email: 'a@b.com' },
  context: {},
};

const baseGroup: GroupMessage = {
  messageId: 'msg-3',
  timestamp: new Date(3000),
  userId: 'u1',
  anonymousId: 'a1',
  groupId: 'org-1',
  traits: { plan: 'pro' },
  context: {},
};

const baseAlias: AliasMessage = {
  messageId: 'msg-4',
  timestamp: new Date(4000),
  userId: 'new-user',
  previousId: 'old-anon',
  context: {},
};

describe('mapper', () => {
  it('mapTrackMessage sets event_type and user fields', () => {
    const ev = mapTrackMessage(baseTrack);
    expect(ev.event_type).toBe('Button Clicked');
    expect(ev.user_id).toBe('u1');
    expect(ev.device_id).toBe('a1');
    expect(ev.time).toBe(1000);
    expect(ev.insert_id).toBe('msg-1');
    expect(ev.event_properties).toEqual({ label: 'signup' });
  });

  it('mapIdentifyMessage uses $identify event type', () => {
    const ev = mapIdentifyMessage(baseIdentify);
    expect(ev.event_type).toBe('$identify');
    expect(ev.user_id).toBe('u1');
    expect(ev.user_properties).toEqual({ $set: { email: 'a@b.com' } });
  });

  it('mapGroupMessage uses $groupidentify event type', () => {
    const ev = mapGroupMessage(baseGroup);
    expect(ev.event_type).toBe('$groupidentify');
    expect(ev.group_type).toBe('group');
    expect(ev.group_value).toBe('org-1');
    expect(ev.group_properties).toEqual({ $set: { plan: 'pro' } });
  });

  it('mapAliasMessage links previousId as device_id', () => {
    const ev = mapAliasMessage(baseAlias);
    expect(ev.event_type).toBe('$identify');
    expect(ev.user_id).toBe('new-user');
    expect(ev.device_id).toBe('old-anon');
  });

  it('mapTrackMessage applies context ip_address to ip field', () => {
    const msg: TrackMessage = {
      ...baseTrack,
      context: { ipAddress: '1.2.3.4' },
    };
    const ev = mapTrackMessage(msg);
    expect(ev.ip).toBe('1.2.3.4');
  });

  it('mapTrackMessage applies context locale to language field', () => {
    const msg: TrackMessage = {
      ...baseTrack,
      context: { locale: 'en-US' },
    };
    const ev = mapTrackMessage(msg);
    expect(ev.language).toBe('en-US');
  });
});

describe('truncateString', () => {
  it('returns short strings unchanged', () => {
    expect(truncateString('hello')).toBe('hello');
  });

  it('truncates strings longer than 1024 code points', () => {
    const long = 'a'.repeat(1025);
    const result = truncateString(long);
    expect(result).toHaveLength(1024);
  });

  it('handles multi-codepoint emoji correctly', () => {
    // Each emoji is 1 code point but 2 UTF-16 units
    const emoji = '😀'.repeat(1025);
    const result = truncateString(emoji);
    expect([...result]).toHaveLength(1024);
  });
});

describe('AmplitudeProvider', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) } as Response),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('page throws ErrUnsupportedOperation', async () => {
    const provider = new AmplitudeProvider({ apiKey: 'test', flushIntervalMs: 0 });
    await expect(
      provider.page({
        messageId: 'm',
        timestamp: new Date(),
        userId: 'u',
        anonymousId: 'a',
        name: '/home',
        properties: {},
        context: {},
      }),
    ).rejects.toThrow(ErrUnsupportedOperation.message);
    await provider.shutdown();
  });

  it('track enqueues and flush sends to Amplitude endpoint', async () => {
    const mockFetch = vi.fn().mockResolvedValue({ ok: true } as Response);
    vi.stubGlobal('fetch', mockFetch);

    const provider = new AmplitudeProvider({
      apiKey: 'test-key',
      flushIntervalMs: 0,
      maxRetries: 0,
    });
    await provider.track(baseTrack);
    await provider.flush();

    expect(mockFetch).toHaveBeenCalledWith(
      'https://api2.amplitude.com/batch',
      expect.objectContaining({ method: 'POST' }),
    );
    const body = JSON.parse((mockFetch.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.api_key).toBe('test-key');
    expect(body.events[0].event_type).toBe('Button Clicked');

    await provider.shutdown();
  });

  it('identify sends synchronously without going through the queue', async () => {
    const mockFetch = vi.fn().mockResolvedValue({ ok: true } as Response);
    vi.stubGlobal('fetch', mockFetch);

    const provider = new AmplitudeProvider({ apiKey: 'k', flushIntervalMs: 0, maxRetries: 0 });
    await provider.identify(baseIdentify);
    expect(mockFetch).toHaveBeenCalledTimes(1);

    const body = JSON.parse((mockFetch.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.events[0].event_type).toBe('$identify');

    await provider.shutdown();
  });

  it('alias sends synchronously', async () => {
    const mockFetch = vi.fn().mockResolvedValue({ ok: true } as Response);
    vi.stubGlobal('fetch', mockFetch);

    const provider = new AmplitudeProvider({ apiKey: 'k', flushIntervalMs: 0, maxRetries: 0 });
    await provider.alias(baseAlias);
    expect(mockFetch).toHaveBeenCalledTimes(1);

    await provider.shutdown();
  });

  it('metadata returns amplitude provider name and capabilities', () => {
    const provider = new AmplitudeProvider({ apiKey: 'k', flushIntervalMs: 0 });
    const meta = provider.metadata();
    expect(meta.name).toBe('amplitude');
    expect(meta.capabilities.track).toBe(true);
    expect(meta.capabilities.page).toBe(false);
    provider.shutdown().catch(() => {});
  });
});
