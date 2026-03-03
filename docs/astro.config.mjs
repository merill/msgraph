import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://graph.pm',
  integrations: [
    starlight({
      title: 'Microsoft Graph Agent Skill',
      description: 'An agent skill that enables AI coding assistants to authenticate to Microsoft 365 and query the Microsoft Graph API. Supports delegated and app-only auth with client secret, certificate, managed identity, and workload identity federation.',
      favicon: '/favicon.ico',
      logo: {
        src: './src/assets/msgraph-skill.svg',
        alt: 'msgraph logo',
      },
      head: [
        // Open Graph
        { tag: 'meta', attrs: { property: 'og:image', content: 'https://graph.pm/og-image.png' } },
        { tag: 'meta', attrs: { property: 'og:image:width', content: '1600' } },
        { tag: 'meta', attrs: { property: 'og:image:height', content: '840' } },
        { tag: 'meta', attrs: { property: 'og:image:type', content: 'image/png' } },
        { tag: 'meta', attrs: { property: 'og:type', content: 'website' } },
        { tag: 'meta', attrs: { property: 'og:site_name', content: 'Microsoft Graph Agent Skill' } },
        // Twitter Card
        { tag: 'meta', attrs: { name: 'twitter:card', content: 'summary_large_image' } },
        { tag: 'meta', attrs: { name: 'twitter:image', content: 'https://graph.pm/og-image.png' } },
        // SEO
        { tag: 'meta', attrs: { name: 'keywords', content: 'Microsoft Graph, agent skill, AI agent, Microsoft 365, Entra ID, MSAL, Graph API, coding assistant, CLI, OpenAPI, delegated auth, app-only auth, managed identity, workload identity, client certificate' } },
      ],
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/merill/msgraph' },
      ],
      components: {
        Header: './src/components/Header.astro',
        ThemeSelect: './src/components/ThemeToggle.astro',
      },
      customCss: ['./src/styles/custom.css'],
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Introduction', slug: 'getting-started/introduction' },
            { label: 'Installation', slug: 'getting-started/installation' },
          ],
        },
        {
          label: 'Usage',
          items: [
            { label: 'Authentication', slug: 'usage/authentication' },
            { label: 'Graph API Calls', slug: 'usage/graph-api-calls' },
            { label: 'OpenAPI Search', slug: 'usage/openapi-search' },
          ],
        },
        {
          label: 'Agent Skill',
          items: [
            { label: 'Skill Setup', slug: 'skill/setup' },
            { label: 'How It Works', slug: 'skill/how-it-works' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'CLI Reference', slug: 'reference/cli' },
            { label: 'Configuration', slug: 'reference/configuration' },
          ],
        },
        {
          label: 'FAQ',
          slug: 'faq',
        },
      ],
    }),
  ],
});
