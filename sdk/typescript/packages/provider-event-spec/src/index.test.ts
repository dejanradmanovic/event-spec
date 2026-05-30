import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { EventSpecProvider } from './index';
import type {
  TrackMessage,
  IdentifyMessage,
  GroupMessage,
  PageMessage,
  AliasMessage,
} from '@dejanradmanovic/event-spec-api';

const BASE_URL = 'https://events.example.com';
const API_KEY = 'test-key';
const SOURCE = 'test-source';

function makeProvider(overrides: Partial<ConstructorParameters<typeof EventSpecProvider>[0]> = {}) {
  return new EventSpecProvider({
    baseURL: BASE_URL,
    apiKey: API_KEY,
    source: SOURCE,
    retryConfig: { maxRetries: 0 },
    ...overrides,
  });
}

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

const basePage: PageMessage = {
  messageId: 'msg-5',
  timestamp: new Date(5000),
  userId: 'u1',
  anonymousId: 'a1',
  name: '/home',
  properties: { title: 'Home' },
  context: {},
};

const baseAlias: AliasMessage = {
  messageId: 'msg-4',
  timestamp: new Date(4000),
  userId: 'new-user',
  previousId: 'old-anon',
  context: {},
};

describe('EventSpecProvider', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) } as Response);
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // ── metadata ──────────────────────────────────────────────────────────────

  it('metadata returns event-spec name and all five capabilities', () => {
    const p = makeProvider();
    const meta = p.metadata();
    expect(meta.name).toBe('event-spec');
    expect(meta.version).toBeTruthy();
    expect(meta.capabilities).toEqual({
      track: true,
      identify: true,
      group: true,
      page: true,
      alias: true,
    });
  });

  it('hooks returns empty array', () => {
    expect(makeProvider().hooks()).toEqual([]);
  });

  // ── track ─────────────────────────────────────────────────────────────────

  it('track POSTs to /v1/track with auth header', async () => {
    const p = makeProvider();
    await p.track(baseTrack);

    expect(mockFetch).toHaveBeenCalledWith(
      `${BASE_URL}/v1/track`,
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({ Authorization: `Bearer ${API_KEY}` }),
      }),
    );
    const body = JSON.parse((mockFetch.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.source).toBe(SOURCE);
    expect(body.event_name).toBe('Button Clicked');
    expect(body.context.user_id).toBe('u1');
    expect(body.context.anonymous_id).toBe('a1');
    expect(body.properties).toEqual({ label: 'signup' });
  });

  // ── identify ──────────────────────────────────────────────────────────────

  it('identify POSTs to /v1/identify with user_id top-level and anonymous_id in context', async () => {
    const p = makeProvider();
    await p.identify(baseIdentify);

    expect(mockFetch).toHaveBeenCalledWith(
      `${BASE_URL}/v1/identify`,
      expect.objectContaining({ method: 'POST' }),
    );
    const body = JSON.parse((mockFetch.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.source).toBe(SOURCE);
    expect(body.user_id).toBe('u1');
    expect(body.traits).toEqual({ email: 'a@b.com' });
    // anonymous_id must be in context, not at the top level.
    expect(body.anonymous_id).toBeUndefined();
    expect(body.context.anonymous_id).toBe('a1');
  });

  // ── group ─────────────────────────────────────────────────────────────────

  it('group POSTs to /v1/group with group_id and identity in context', async () => {
    const p = makeProvider();
    await p.group(baseGroup);

    expect(mockFetch).toHaveBeenCalledWith(
      `${BASE_URL}/v1/group`,
      expect.objectContaining({ method: 'POST' }),
    );
    const body = JSON.parse((mockFetch.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.group_id).toBe('org-1');
    // user_id and anonymous_id must be in context, not at the top level.
    expect(body.user_id).toBeUndefined();
    expect(body.anonymous_id).toBeUndefined();
    expect(body.context.user_id).toBe('u1');
    expect(body.context.anonymous_id).toBe('a1');
  });

  // ── page ──────────────────────────────────────────────────────────────────

  it('page POSTs to /v1/page with name and identity in context', async () => {
    const p = makeProvider();
    await p.page(basePage);

    expect(mockFetch).toHaveBeenCalledWith(
      `${BASE_URL}/v1/page`,
      expect.objectContaining({ method: 'POST' }),
    );
    const body = JSON.parse((mockFetch.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.name).toBe('/home');
    // user_id and anonymous_id must be in context, not at the top level.
    expect(body.user_id).toBeUndefined();
    expect(body.anonymous_id).toBeUndefined();
    expect(body.context.user_id).toBe('u1');
    expect(body.context.anonymous_id).toBe('a1');
  });

  // ── alias ─────────────────────────────────────────────────────────────────

  it('alias POSTs to /v1/alias with previous_id', async () => {
    const p = makeProvider();
    await p.alias(baseAlias);

    expect(mockFetch).toHaveBeenCalledWith(
      `${BASE_URL}/v1/alias`,
      expect.objectContaining({ method: 'POST' }),
    );
    const body = JSON.parse((mockFetch.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.user_id).toBe('new-user');
    expect(body.previous_id).toBe('old-anon');
  });

  // ── flush ─────────────────────────────────────────────────────────────────

  it('flush POSTs to /v1/flush with source', async () => {
    const p = makeProvider();
    await p.flush();

    expect(mockFetch).toHaveBeenCalledWith(
      `${BASE_URL}/v1/flush`,
      expect.objectContaining({ method: 'POST' }),
    );
    const body = JSON.parse((mockFetch.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.source).toBe(SOURCE);
  });

  // ── shutdown ──────────────────────────────────────────────────────────────

  it('shutdown calls flush then throws on subsequent calls', async () => {
    const p = makeProvider();
    await p.shutdown();

    // flush was sent
    const body = JSON.parse((mockFetch.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.source).toBe(SOURCE);

    // subsequent calls throw
    await expect(p.track(baseTrack)).rejects.toThrow('shut down');
    await expect(p.flush()).rejects.toThrow('shut down');
    await expect(p.shutdown()).rejects.toThrow('shut down');
  });

  // ── auth header on every request ─────────────────────────────────────────

  it('all call types include Authorization: Bearer header', async () => {
    const p = makeProvider();
    await p.track(baseTrack);
    await p.identify(baseIdentify);
    await p.group(baseGroup);
    await p.page(basePage);
    await p.alias(baseAlias);
    await p.flush();

    for (const call of mockFetch.mock.calls as [string, RequestInit][]) {
      expect(call[1].headers).toEqual(
        expect.objectContaining({ Authorization: `Bearer ${API_KEY}` }),
      );
    }
  });

  // ── context attributes ────────────────────────────────────────────────────

  it('track forwards user_agent and ip_address into context.attributes', async () => {
    const p = makeProvider();
    await p.track({
      ...baseTrack,
      context: { userAgent: 'TestBot/1.0', ipAddress: '1.2.3.4', extra: { session_id: 'sess-1' } },
    });

    const body = JSON.parse((mockFetch.mock.calls[0] as [string, RequestInit])[1].body as string);
    expect(body.context.attributes.user_agent).toBe('TestBot/1.0');
    expect(body.context.attributes.ip_address).toBe('1.2.3.4');
    expect(body.context.attributes.session_id).toBe('sess-1');
  });

  // ── proxy rewriting ───────────────────────────────────────────────────────

  it('reverse_proxy rewrites the host to the proxy URL', async () => {
    const p = makeProvider({
      baseURL: 'https://events.example.com',
      proxyUrl: 'https://proxy.internal',
      proxyMode: 'reverse_proxy',
    });
    await p.track(baseTrack);

    const calledURL = (mockFetch.mock.calls[0] as [string])[0];
    expect(calledURL).toMatch(/^https:\/\/proxy\.internal/);
    expect(calledURL).toContain('/v1/track');
  });

  // ── error propagation ─────────────────────────────────────────────────────

  it('propagates non-ok HTTP responses as errors', async () => {
    mockFetch.mockResolvedValue({ ok: false, status: 400 } as Response);
    const p = makeProvider();
    await expect(p.track(baseTrack)).rejects.toThrow('400');
  });
});
