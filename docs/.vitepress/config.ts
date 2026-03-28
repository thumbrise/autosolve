import {defineConfig} from 'vitepress'

export default defineConfig({
  title: 'autosolve',
  description: 'Self-hosted Go daemon that polls GitHub repos and dispatches AI agents to solve issues. No webhooks, no CI glue.',
  base: '/autosolve/',
  sitemap: {
    hostname: 'https://thumbrise.github.io/autosolve/',
  },
  head: [
    ['meta', {
      name: 'keywords',
      content: 'github issue automation self-hosted, golang long running service retry, golang graceful degraded mode, opentelemetry golang worker metrics'
    }],
  ],

  themeConfig: {
    nav: [
      {text: 'Guide', link: '/guide/getting-started'},
      {text: 'Internals', link: '/internals/overview'},
      {text: 'Contributing', link: '/contributing/adding-worker'},
      {text: 'Devlog', link: '/devlog/'},
      {text: 'The Idea', link: '/project/idea'},
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'User Guide',
          items: [
            {text: 'Getting Started', link: '/guide/getting-started'},
            {text: 'Configuration', link: '/guide/configuration'},
            {text: 'Observability', link: '/guide/observability'},
          ],
        },
      ],
      '/internals/': [
        {
          text: 'Internals',
          items: [
            {text: 'Architecture Overview', link: '/internals/overview'},
            {text: 'Two-Phase Scheduler', link: '/internals/two-phase'},
            {text: 'Error Handling & Retry', link: '/internals/error-handling'},
            {text: 'longrun Package', link: '/internals/longrun'},
          ],
        },
      ],
      '/contributing/': [
        {
          text: 'Contributing',
          items: [
            {text: 'Adding a Worker', link: '/contributing/adding-worker'},
            {text: 'Adding an Integration', link: '/contributing/adding-integration'},
          ],
        },
      ],
      '/devlog/': [
        {
          text: 'Devlog',
          items: [
            {text: 'About This Devlog', link: '/devlog/'},
            {text: '#1 — Why Polling', link: '/devlog/001-why-polling'},
            {text: '#2 — Graceful Shutdown Is Hard', link: '/devlog/002-graceful-shutdown'},
            {text: '#3 — Building longrun', link: '/devlog/003-building-longrun'},
            {text: '#4 — From God Table to sqlc', link: '/devlog/004-god-table-to-sqlc'},
            {text: '#5 — Two-Phase Scheduler', link: '/devlog/005-two-phase-scheduler'},
            {text: '#6 — Over-Engineered Event Bus', link: '/devlog/006-over-engineered-event-bus'},
            {text: '#7 — DX & Inner Loop', link: '/devlog/007-dx-inner-loop'},
            {text: '#8 — goqite over hand-rolled jobs', link: '/devlog/008-goqite-over-hand-rolled-jobs'},
          ],
        },
      ],
      '/project/': [
        {
          text: 'Project',
          items: [
            {text: 'The Idea', link: '/project/idea'},
            {text: 'Status & Roadmap', link: '/project/status'},
            {text: 'Ideas & Wishlist', link: '/project/ideas'},
          ],
        },
      ],
    },

    socialLinks: [
      {icon: 'github', link: 'https://github.com/thumbrise/autosolve'},
    ],

    editLink: {
      pattern: 'https://github.com/thumbrise/autosolve/edit/main/docs/:path',
    },

    footer: {
      message: 'Apache 2.0 · Built in public · Contributions welcome',
    },
  },
})
