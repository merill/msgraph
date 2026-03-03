import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  integrations: [
    starlight({
      title: 'Microsoft Graph Agent Skill',
      description: 'Microsoft Graph API agent skill for AI coding assistants',
      favicon: '/favicon.ico',
      logo: {
        src: './src/assets/msgraph-skill.svg',
        alt: 'msgraph logo',
      },
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/merill/msgraph' },
      ],
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
