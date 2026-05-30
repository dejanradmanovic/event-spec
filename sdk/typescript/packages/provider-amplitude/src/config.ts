export const DEFAULT_ENDPOINT = 'https://api2.amplitude.com/batch';

export type ProxyMode = 'direct' | 'reverse_proxy' | 'custom';
export type OverflowPolicy = 'drop_oldest' | 'drop_newest';

export interface AmplitudeConfig {
  apiKey: string;
  endpoint?: string;

  // Proxy settings — override the destination URL to bypass ad-blockers.
  // 'reverse_proxy': replace scheme+host with proxyUrl, keep the provider path.
  // 'custom': use proxyUrl as the complete replacement URL.
  proxyUrl?: string;
  proxyMode?: ProxyMode;

  // Batching & queue
  batchSize?: number;
  flushIntervalMs?: number;
  maxQueueSize?: number;
  overflowPolicy?: OverflowPolicy;

  // Retry & backoff
  maxRetries?: number;
  initialBackoffMs?: number;
  maxBackoffMs?: number;
  retryMultiplier?: number;
  jitter?: boolean;

  // Rate limiting
  requestsPerSecond?: number;
}
