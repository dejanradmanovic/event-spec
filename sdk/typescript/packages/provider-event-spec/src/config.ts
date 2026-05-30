import type { ProviderConfig } from '@dejanradmanovic/event-spec-api';

export interface EventSpecConfig extends ProviderConfig {
  baseURL: string;
  apiKey: string;
  source: string;
}
