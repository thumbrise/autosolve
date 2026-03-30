---
title: autosolve Status & Roadmap
description: Current project status, what works today, the v1 architecture epic, tech stack, and planned features for the autosolve GitHub automation daemon.
---

# Status & Roadmap

::: warning Active Development
autosolve is being built right now. APIs may change, features may shift. This is a feature, not a bug — your input shapes the project.
:::

## What Works Today

- **Multi-repo GitHub issue polling** with per-repo state persistence in SQLite
- **Two-phase scheduler** — preflights validate repos, then workers start polling
- **Outbox relay** — outbox events relayed to goqite job queue
- **AI dispatch** — IssueExplainer consumes queue, calls Ollama, logs classification
- **Global workers** — interval tasks not scoped to a repository (e.g. queue consumers)
- **Per-error retry with exponential backoff** via the `longrun` package
- **Degraded mode** — unknown errors don't crash workers, they retry with loud logging
- **Rate limiting** via HTTP transport layer (transparent to domain code)
- **Full OpenTelemetry observability** — traces, metrics, logs via OTLP/gRPC
- **OTEL metrics** on OutboxRelay and IssueExplainer — throughput, latency, backpressure
- **goose migrations + sqlc-generated DAL** — type-safe database layer
- **Google Wire DI** — clean dependency graph, no magic

## Current Epic

[Epic: v1 architecture redesign](https://github.com/thumbrise/autosolve/issues/59) — stabilizing the core before adding the AI dispatch layer.

## What's Next

These are the broad directions, not promises. Priorities shift based on real usage and feedback.

| Area | What | Status |
|------|------|--------|
| **AI Dispatch** | Queue consumer calls Ollama, classifies issues | Done (#156) |
| **Result Publishing** | Post AI results back to GitHub (comments, PRs) | Next (#157) |
| **Adaptive Polling** | Back off when repos are quiet, speed up when active | [#53](https://github.com/thumbrise/autosolve/issues/53) |
| **longrun extraction** | Extract `pkg/longrun` into a standalone Go module | After API stabilizes |
| **Module system** | Self-contained modules per partition type | When second partition appears |
| **Comment polling** | Watch for `@bot` mentions in comments | Planned |

## Tech Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.26+ |
| Database | SQLite (pure Go, WAL mode) |
| Migrations | [goose](https://github.com/pressly/goose) |
| SQL codegen | [sqlc](https://sqlc.dev) |
| DI | [Wire](https://github.com/google/wire) |
| Observability | [OpenTelemetry](https://opentelemetry.io) (OTLP/gRPC) |
| CLI | [cobra](https://github.com/spf13/cobra) + [viper](https://github.com/spf13/viper) |

## Want to Contribute?

The best time to join is now — while the architecture is fresh and decisions are still being made. See [Adding a Worker](/contributing/adding-worker) for a concrete starting point, or [open an issue](https://github.com/thumbrise/autosolve/issues/new) with your idea.
