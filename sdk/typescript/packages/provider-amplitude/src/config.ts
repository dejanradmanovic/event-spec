export const DEFAULT_ENDPOINT = 'https://api2.amplitude.com/batch';

export interface AmplitudeConfig {
  apiKey: string;
  endpoint?: string;
  batchSize?: number;
  flushIntervalMs?: number;
  maxRetries?: number;
}
