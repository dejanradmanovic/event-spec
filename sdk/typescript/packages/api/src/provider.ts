import type { Hook } from './hooks';

export interface ProviderCapabilities {
  track: boolean;
  identify: boolean;
  group: boolean;
  page: boolean;
  alias: boolean;
}

export interface ProviderMetadata {
  name: string;
  version: string;
  capabilities: ProviderCapabilities;
}

// MessageContext contains environment and device metadata attached to every message.
export interface MessageContext {
  ipAddress?: string;
  userAgent?: string;
  locale?: string;
  timezone?: string;
  library?: Record<string, unknown>;
  app?: Record<string, unknown>;
  device?: Record<string, unknown>;
  os?: Record<string, unknown>;
  network?: Record<string, unknown>;
  screen?: Record<string, unknown>;
  extra?: Record<string, unknown>;
}

export interface TrackMessage {
  messageId: string;
  timestamp: Date;
  eventName: string;
  properties: Record<string, unknown>;
  userId: string;
  anonymousId: string;
  context: MessageContext;
}

export interface IdentifyMessage {
  messageId: string;
  timestamp: Date;
  userId: string;
  anonymousId: string;
  traits: Record<string, unknown>;
  context: MessageContext;
}

export interface GroupMessage {
  messageId: string;
  timestamp: Date;
  userId: string;
  anonymousId: string;
  groupId: string;
  traits: Record<string, unknown>;
  context: MessageContext;
}

export interface PageMessage {
  messageId: string;
  timestamp: Date;
  userId: string;
  anonymousId: string;
  name: string;
  properties: Record<string, unknown>;
  context: MessageContext;
}

export interface AliasMessage {
  messageId: string;
  timestamp: Date;
  userId: string;
  previousId: string;
  context: MessageContext;
}

// ErrUnsupportedOperation is returned by provider methods that have no equivalent
// in the underlying vendor API, preventing silent data loss.
export const ErrUnsupportedOperation = new Error('unsupported provider operation');

// Provider is the interface every analytics destination adapter must satisfy.
// The single interface is intentional — providers that don't support a method
// should throw ErrUnsupportedOperation rather than silently no-oping.
export interface Provider {
  metadata(): ProviderMetadata;
  hooks(): Hook[];
  track(msg: TrackMessage): Promise<void>;
  identify(msg: IdentifyMessage): Promise<void>;
  group(msg: GroupMessage): Promise<void>;
  page(msg: PageMessage): Promise<void>;
  alias(msg: AliasMessage): Promise<void>;
  flush(): Promise<void>;
  shutdown(): Promise<void>;
}
