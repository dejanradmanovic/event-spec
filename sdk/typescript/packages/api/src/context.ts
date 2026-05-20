export interface AnalyticsContext {
  userId?: string;
  anonymousId?: string;
  attributes?: Record<string, unknown>;
}

// merge combines base and override AnalyticsContexts.
// Non-empty override fields win. Attributes merged key-by-key with override keys winning.
export function merge(base: AnalyticsContext, override: AnalyticsContext): AnalyticsContext {
  const result: AnalyticsContext = {
    userId: override.userId || base.userId,
    anonymousId: override.anonymousId || base.anonymousId,
  };

  if (base.attributes || override.attributes) {
    result.attributes = { ...base.attributes, ...override.attributes };
  }

  return result;
}
