import { defineConfig } from 'vitepress';

export default defineConfig({
  title: 'TokenMeter',
  description: 'AI 编码 Agent 的本地用量仪表盘',
  base: '/',
  cleanUrls: true,
  ignoreDeadLinks: true,
  head: [
    ['meta', { name: 'theme-color', content: '#7C3AED' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:title', content: 'TokenMeter Documentation' }],
    ['meta', { property: 'og:description', content: 'Local usage, cost, and activity observability for AI coding agents.' }]
  ],
  themeConfig: {
    nav: [
      { text: 'Guide', link: '/guide/getting-started' },
      { text: 'Sources', link: '/sources/' },
      { text: 'Configuration', link: '/configuration/environment-variables' },
      { text: 'Changelog', link: '/changelog/v1.2.0' },
      { text: 'GitHub', link: 'https://github.com/tt-a1i/tokenmeter' }
    ],
    sidebar: [
      {
        text: 'Guide',
        items: [
          { text: 'Getting Started', link: '/guide/getting-started' },
          { text: 'Installation', link: '/guide/installation' },
          { text: 'CLI Reference', link: '/guide/cli-reference' },
          { text: 'JSON Output', link: '/guide/json-output' },
          { text: 'Blocks and Statusline', link: '/guide/blocks-and-statusline' },
          { text: 'Migrating to v1', link: '/guide/migration-v1' }
        ]
      },
      {
        text: 'Sources',
        items: [
          { text: 'Source Index', link: '/sources/' },
          { text: 'Claude Code', link: '/sources/claude' },
          { text: 'Codex', link: '/sources/codex' },
          { text: 'OpenCode', link: '/sources/opencode' },
          { text: 'Amp', link: '/sources/amp' },
          { text: 'Droid', link: '/sources/droid' },
          { text: 'Codebuff', link: '/sources/codebuff' },
          { text: 'Hermes', link: '/sources/hermes' },
          { text: 'pi-agent', link: '/sources/pi' },
          { text: 'Goose', link: '/sources/goose' },
          { text: 'Kilo', link: '/sources/kilo' },
          { text: 'Kimi', link: '/sources/kimi' },
          { text: 'OpenClaw', link: '/sources/openclaw' },
          { text: 'Qwen', link: '/sources/qwen' },
          { text: 'GitHub Copilot CLI', link: '/sources/copilot' },
          { text: 'Gemini CLI', link: '/sources/gemini' }
        ]
      },
      {
        text: 'Configuration',
        items: [
          { text: 'Environment Variables', link: '/configuration/environment-variables' },
          { text: 'Unified Config', link: '/configuration/unified-config' },
          { text: 'Statusline', link: '/configuration/statusline' },
          { text: 'Budgets', link: '/configuration/budgets' },
          { text: 'Webhooks', link: '/configuration/webhooks' }
        ]
      },
      {
        text: 'Integration',
        items: [
          { text: 'Claude Hooks', link: '/integration/claude-hooks' }
        ]
      },
      {
        text: 'Changelog',
        items: [
          { text: 'v1.2.0', link: '/changelog/v1.2.0' }
        ]
      }
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/tt-a1i/tokenmeter' }
    ],
    search: {
      provider: 'local'
    },
    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © TokenMeter contributors'
    }
  }
});
