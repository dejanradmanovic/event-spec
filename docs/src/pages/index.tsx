import React from 'react';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import CodeBlock from '@theme/CodeBlock';
import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

import styles from './index.module.css';

const SPEC_YAML = `# specs/ecommerce/product_viewed/1-0-0.yaml
$schema: "https://event-spec.io/schemas/event/v1"
name: product_viewed
display_name: "Product Viewed"
version: "1-0-0"
status: active
namespace: ecommerce
type: track
event_name: "Product Viewed"

properties:
  product_id:
    type: string
    required: true
  category:
    type: string
    required: true
    enum: [clothing, electronics, other]
  currency:
    type: string
    required: false
    default: "USD"`;

const GO_USAGE = `import (
    core "github.com/dejanradmanovic/event-spec/analytics"
    "github.com/dejanradmanovic/event-spec/provider/amplitude"
    generated "your-module/generated"
)

amp, _ := amplitude.New(amplitude.Config{
    ProviderConfig: provider.ProviderConfig{
        APIKey:     "\${AMPLITUDE_API_KEY}",
        SecretType: provider.SecretEnvVar,
    },
})

client := core.NewClient(core.WithProviders(amp))
es := generated.New(client)

es.ProductViewed(ctx, generated.ProductViewedProperties{
    Category:  generated.ProductViewedCategoryElectronics,
    ProductId: "SKU-123",
})`;

const TS_USAGE = `import { Client } from '@dejanradmanovic/event-spec-api';
import { AmplitudeProvider } from '@dejanradmanovic/event-spec-provider-amplitude';
import { productViewed, ProductViewedCategory } from './generated/product_viewed';

const amp = new AmplitudeProvider({ apiKey: process.env.AMPLITUDE_API_KEY! });
const client = new Client({ providers: [amp] });

await client.productViewed({
    category: ProductViewedCategory.Electronics,
    productId: 'SKU-123',
});`;

const KOTLIN_USAGE = `import io.eventspec.analytics.Client
import io.eventspec.analytics.ClientOptions
import io.eventspec.analytics.amplitude.AmplitudeConfig
import io.eventspec.analytics.amplitude.AmplitudeProvider
import analytics.EventSpec
import analytics.ProductViewedProperties
import analytics.ProductViewedCategory

val amp = AmplitudeProvider(AmplitudeConfig(apiKey = System.getenv("AMPLITUDE_API_KEY")!!))
val client = Client(ClientOptions(providers = listOf(amp)))
val es = EventSpec(client)

es.productViewed(ProductViewedProperties(
    category = ProductViewedCategory.ELECTRONICS,
    productId = "SKU-123",
))`;

const FEATURES = [
  {
    icon: '📋',
    title: 'Event Contract Layer',
    description:
      'Define events once in YAML with versioning, JSON Schema validation, and breaking-change detection using SchemaVer.',
  },
  {
    icon: '🔌',
    title: 'SDK Runtime Layer',
    description:
      'Pluggable analytics destinations behind a stable Provider interface. Hooks, context propagation, queueing, and dispatch included.',
  },
  {
    icon: '⚡',
    title: 'Codegen Layer',
    description:
      'Generate language-native typed wrappers from your event registry for Go, TypeScript, and Kotlin (Swift, Python planned).',
  },
  {
    icon: '🏛️',
    title: 'Governance Layer',
    description:
      'Registry server with REST API, audit tooling to scan codebases for event usage, and automatic catalog generation.',
  },
];

const PROVIDERS = [
  { name: 'Amplitude', available: true },
  { name: 'PostHog', available: false },
  { name: 'Mixpanel', available: false },
  { name: 'Segment', available: false },
  { name: 'GA4', available: false },
  { name: 'RudderStack', available: false },
];

const SDKS = [
  { name: 'Go', available: true },
  { name: 'TypeScript', available: true },
  { name: 'Kotlin', available: true },
  { name: 'Swift', available: false },
  { name: 'Python', available: false },
  { name: 'Rust', available: false },
];

export default function Home(): React.ReactElement {
  const { siteConfig } = useDocusaurusContext();

  return (
    <Layout title={siteConfig.title} description={siteConfig.tagline}>
      {/* Hero */}
      <div className="hero-banner">
        <h1>{siteConfig.tagline}</h1>
        <p className="sub">
          Stop coupling instrumentation to a single analytics vendor. event-spec lets you define
          events once in YAML, generate type-safe wrappers for every language, and swap providers
          without touching application code.
        </p>
        <div className="hero-actions">
          <Link className="button button--primary button--lg" to="/docs/getting-started">
            Get Started →
          </Link>
          <Link
            className="button button--secondary button--lg"
            href="https://github.com/dejanradmanovic/event-spec"
          >
            GitHub ↗
          </Link>
        </div>
      </div>

      {/* How it works */}
      <section className="features-section">
        <div className="features-grid">
          {FEATURES.map((f) => (
            <div key={f.title} className="feature-card">
              <span className="feature-icon">{f.icon}</span>
              <h3>{f.title}</h3>
              <p>{f.description}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Provider ecosystem */}
      <section className="providers-section">
        <h2>Provider Ecosystem</h2>
        <div className="provider-grid">
          {PROVIDERS.map((p) => (
            <div key={p.name} className={`provider-chip ${p.available ? 'available' : 'coming-soon'}`}>
              <span>{p.available ? '✓' : '○'}</span>
              <span>{p.name}</span>
              {!p.available && <span style={{ fontSize: '0.75rem', opacity: 0.7 }}>soon</span>}
            </div>
          ))}
        </div>
      </section>

      {/* SDK strip */}
      <section className="sdks-section">
        <h2>SDK Support</h2>
        <div className="sdk-strip">
          {SDKS.map((s) => (
            <span key={s.name} className={`sdk-badge ${s.available ? 'available' : 'coming-soon'}`}>
              {s.available ? '✓' : '○'} {s.name}
            </span>
          ))}
        </div>
      </section>

      {/* Quick code sample */}
      <section className="code-section">
        <h2>From spec to instrumented in minutes</h2>
        <div className="code-section-inner">
          <Tabs>
            <TabItem value="spec" label="1. Define">
              <CodeBlock language="yaml" title="specs/ecommerce/product_viewed/1-0-0.yaml">
                {SPEC_YAML}
              </CodeBlock>
            </TabItem>
            <TabItem value="generate" label="2. Generate">
              <CodeBlock language="bash" title="Terminal">
                {`# Generate Go wrappers\nevent-spec generate --lang go --out ./generated\n\n# Generate TypeScript wrappers\nevent-spec generate --lang typescript --out ./src/analytics/generated\n\n# Generate Kotlin wrappers\nevent-spec generate --lang kotlin --out ./generated`}
              </CodeBlock>
            </TabItem>
            <TabItem value="go" label="3. Use (Go)">
              <CodeBlock language="go" title="main.go">
                {GO_USAGE}
              </CodeBlock>
            </TabItem>
            <TabItem value="ts" label="3. Use (TypeScript)">
              <CodeBlock language="typescript" title="index.ts">
                {TS_USAGE}
              </CodeBlock>
            </TabItem>
            <TabItem value="kotlin" label="3. Use (Kotlin)">
              <CodeBlock language="kotlin" title="main.kt">
                {KOTLIN_USAGE}
              </CodeBlock>
            </TabItem>
          </Tabs>
        </div>
      </section>
    </Layout>
  );
}
