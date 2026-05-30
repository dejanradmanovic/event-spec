import type { ProviderConfig } from '@dejanradmanovic/event-spec-api';

// ServerProviderConfig extends the shared ProviderConfig with event-spec server fields.
export interface ServerProviderConfig extends ProviderConfig {
  baseURL: string;
  apiKey: string;
  source: string;
}
