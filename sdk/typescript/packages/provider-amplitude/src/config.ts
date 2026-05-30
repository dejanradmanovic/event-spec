import type { ProviderConfig } from '@dejanradmanovic/event-spec-api';

export const DEFAULT_ENDPOINT = 'https://api2.amplitude.com/batch';

// AmplitudeConfig extends the shared ProviderConfig with Amplitude-specific fields.
export interface AmplitudeConfig extends ProviderConfig {
  apiKey: string;
  endpoint?: string;
}
