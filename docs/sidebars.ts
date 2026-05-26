import type { SidebarsConfig } from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  gettingStartedSidebar: [
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      items: [
        'getting-started/index',
        'getting-started/installation',
        'getting-started/quickstart',
        'getting-started/workspace',
      ],
    },
    {
      type: 'category',
      label: 'Concepts',
      collapsed: false,
      items: [
        'concepts/event-contract',
        'concepts/registry',
        'concepts/providers',
        'concepts/hooks',
        'concepts/context',
        'concepts/codegen',
      ],
    },
    {
      type: 'category',
      label: 'Hooks',
      items: [
        'hooks/index',
        'hooks/sampling',
        'hooks/validation',
        'hooks/custom',
      ],
    },
    {
      type: 'category',
      label: 'Registry',
      items: [
        'registry/local',
        'registry/git',
        'registry/server',
      ],
    },
    {
      type: 'category',
      label: 'Spec Reference',
      items: [
        'spec-reference/event',
        'spec-reference/source',
        'spec-reference/destination',
        'spec-reference/workspace',
      ],
    },
    {
      type: 'category',
      label: 'Contributing',
      items: [
        'contributing/index',
        'contributing/adding-providers',
        'contributing/adding-sdks',
      ],
    },
  ],

  serverSidebar: [
    {
      type: 'category',
      label: 'Server',
      collapsed: false,
      items: [
        'server/index',
        'server/docker',
        'server/relay',
        'server/authentication',
        'server/api-reference',
        'server/configuration',
        'server/admin-ui',
      ],
    },
  ],

  ciIntegrationsSidebar: [
    {
      type: 'category',
      label: 'CI Integrations',
      collapsed: false,
      items: [
        'ci-integrations/index',
        'ci-integrations/github-actions',
        'ci-integrations/docker',
      ],
    },
  ],

  cliSidebar: [
    {
      type: 'category',
      label: 'CLI Reference',
      collapsed: false,
      items: [
        'cli/index',
        'cli/new',
        'cli/validate',
        'cli/diff',
        'cli/generate',
        'cli/pull',
        'cli/audit',
        'cli/docs',
        'cli/serve',
        'cli/publish',
        'cli/admin',
      ],
    },
  ],

  sdkSidebar: [
    {
      type: 'category',
      label: 'SDKs',
      collapsed: false,
      items: [
        'sdks/go',
        'sdks/typescript',
      ],
    },
  ],

  providerSidebar: [
    {
      type: 'category',
      label: 'Providers',
      collapsed: false,
      items: [
        'providers/index',
        'providers/amplitude',
        'providers/noop',
        'providers/custom',
      ],
    },
  ],
};

export default sidebars;
