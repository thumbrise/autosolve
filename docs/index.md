---
layout: home

hero:
  name: autosolve
  text: AI agents for your GitHub issues
  tagline: "Open source Go daemon. Polls repos, dispatches AI agents, posts results. No webhooks, no CI glue — just run and forget."
  actions:
    - theme: brand
      text: Quick Start
      link: /guide/getting-started
    - theme: alt
      text: Read the Idea
      link: /project/idea
    - theme: alt
      text: Devlog
      link: /devlog/
    - theme: alt
      text: GitHub
      link: https://github.com/thumbrise/autosolve

features:
  - icon: 🚧  
    title: Active Development  
    details: "Open source golang tool, built in public. Architecture is stabilizing, ideas are welcome, contributions are encouraged."
  - icon: 🧠
    title: Clear Problem, Clear Intent
    details: "AI dev tools exist but integrating them with GitHub is painful. autosolve is a simple bridge: poll → detect → dispatch → report. Read the full idea."
    link: /project/idea
    linkText: Read the Idea →
  - icon: 🔄
    title: Multi-Repo Polling
    details: One daemon, many repositories. Each repo is an independent partition with its own state, cursor, and rate limiting.
  - icon: 🛡️
    title: Two-Phase Scheduler
    details: Preflights validate access before workers start. If validation fails — workers never launch. Fail fast, fail safe.
    link: /internals/two-phase
    linkText: How it works →
  - icon: 📊
    title: OpenTelemetry Built-in
    details: Traces, metrics, logs via OTLP/gRPC. Every task invocation is a span. Plug into Grafana, Jaeger, or anything OTEL-compatible.
  - icon: 🔌
    title: Extensible by Design
    details: "Add a worker: implement one interface, register in DI, run codegen. The partition system is ready for org-level and user-level scopes."
    link: /contributing/adding-worker
    linkText: Add a worker →
---
