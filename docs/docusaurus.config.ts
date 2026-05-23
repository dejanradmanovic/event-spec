import { themes as prismThemes } from 'prism-react-renderer';
import type { Config } from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'event-spec',
  tagline: 'Define events once. Ship to any analytics provider.',
  favicon: 'img/favicon.svg',

  url: 'https://dejanradmanovic.github.io',
  baseUrl: '/event-spec/',

  organizationName: 'dejanradmanovic',
  projectName: 'event-spec',
  trailingSlash: false,

  onBrokenLinks: 'throw',
  markdown: {
    mermaid: true,
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

  themes: ['@docusaurus/theme-mermaid'],

  plugins: ['docusaurus-plugin-image-zoom'],

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/dejanradmanovic/event-spec/edit/main/docs/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/social-card.png',
    colorMode: {
      defaultMode: 'dark',
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'event-spec',
      logo: {
        alt: 'event-spec logo',
        src: 'img/logo.svg',
        srcDark: 'img/logo-dark.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'gettingStartedSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          type: 'docSidebar',
          sidebarId: 'serverSidebar',
          position: 'left',
          label: 'Server',
        },
        {
          type: 'docSidebar',
          sidebarId: 'cliSidebar',
          position: 'left',
          label: 'CLI',
        },
        {
          type: 'docSidebar',
          sidebarId: 'sdkSidebar',
          position: 'left',
          label: 'SDKs',
        },
        {
          type: 'docSidebar',
          sidebarId: 'providerSidebar',
          position: 'left',
          label: 'Providers',
        },
        {
          href: 'https://github.com/dejanradmanovic/event-spec',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            { label: 'Getting Started', to: '/docs/getting-started' },
            { label: 'CLI Reference', to: '/docs/cli' },
            { label: 'SDK — Go', to: '/docs/sdks/go' },
            { label: 'SDK — TypeScript', to: '/docs/sdks/typescript' },
          ],
        },
        {
          title: 'Concepts',
          items: [
            { label: 'Event Contract', to: '/docs/concepts/event-contract' },
            { label: 'Providers', to: '/docs/concepts/providers' },
            { label: 'Hooks', to: '/docs/concepts/hooks' },
            { label: 'Registry', to: '/docs/concepts/registry' },
          ],
        },
        {
          title: 'More',
          items: [
            { label: 'GitHub', href: 'https://github.com/dejanradmanovic/event-spec' },
            { label: 'Contributing', to: '/docs/contributing' },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} event-spec contributors. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['go', 'bash', 'yaml', 'typescript', 'json'],
    },
    zoom: {
      selector: '.markdown img',
      background: {
        light: 'rgb(255, 255, 255)',
        dark: 'rgb(20, 28, 43)',
      },
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
