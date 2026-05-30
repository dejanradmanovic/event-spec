import type { OverflowPolicy } from './queue';

export type ProxyMode = 'direct' | 'reverse_proxy' | 'custom';

export interface RetryConfig {
  maxRetries?: number;
  initialBackoffMs?: number;
  maxBackoffMs?: number;
  multiplier?: number;
  jitter?: boolean;
}

export interface RateLimitConfig {
  requestsPerSecond?: number;
}

// ProviderConfig holds construction settings common to all provider implementations.
// Providers extend this interface with their own required fields (e.g. apiKey, endpoint).
export interface ProviderConfig {
  // Proxy — override the destination URL to bypass ad-blockers.
  proxyUrl?: string;
  proxyMode?: ProxyMode;

  // Batching & queue
  batchSize?: number;
  flushIntervalMs?: number;
  maxQueueSize?: number;
  overflowPolicy?: OverflowPolicy;

  // Retry & backoff
  retryConfig?: RetryConfig;

  // Rate limiting
  rateLimitConfig?: RateLimitConfig;
}
