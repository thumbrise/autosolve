---
title: "longrun: Roadmap"
description: Where longrun is headed — from embedded package to standalone resilience toolkit.
---

# Roadmap

longrun started as an internal task runner for autosolve. After several redesigns it became something more general — a resilience toolkit with per-error retry, pluggable backoff, persistent state, and a unified failure pipeline.

This page describes where it's going.

## Current State

A flat Go package at `pkg/longrun/`. Zero internal dependencies beyond `golang.org/x/sync` (and OTel for optional observability). Battle-tested inside autosolve — preflights, workers, degraded mode, baseline retry.

The API is stabilizing. The abstractions (`BackoffFunc`, `AttemptStore`, `failureHandler`) are clean and composable. But it's still embedded in the autosolve repo.

## Phase 1: Harden Inside autosolve

Before extraction, the API must prove itself:

- [ ] Run in production across multiple repositories and worker types
- [ ] Validate `AttemptStore` contract with a real persistent implementation (Redis or SQLite)
- [ ] Confirm `BackoffFunc` extensibility — jitter, decorrelated, adaptive strategies in real use
- [ ] No breaking changes for 2+ releases

This is where we are now.

## Phase 2: Extract as `thumbrise/resilience`

When the API is stable, extract into a standalone multi-module repository:

```
thumbrise/resilience/
├── go.mod                  // core module — zero external deps
├── backoff/                // BackoffFunc, Exponential, Constant
├── retry/                  // AttemptStore, MemoryStore, Matcher
├── task/                   // Task, Runner, failureHandler pipeline
├── otel/
│   └── go.mod              // separate module — depends on OTel SDK
└── circuit/
    └── go.mod              // separate module — circuit breaker (future)
```

### Why Multi-Module?

Go modules work at the `go.mod` level. A single `go.mod` means `go mod tidy` downloads everything — even packages you don't use. Their transitive dependencies land in your `go.sum`.

Multi-module (like `go.opentelemetry.io/otel`) isolates heavy dependencies:

- `go get thumbrise/resilience/task` → pulls core + task. No OTel SDK.
- `go get thumbrise/resilience/otel` → pulls core + OTel bindings. Only if you want tracing.

Users import exactly what they need. No bloat.

### Core Module: Zero Dependencies

The core (`thumbrise/resilience`) would contain only fundamental abstractions:

- `BackoffFunc` — pure function type
- `Exponential`, `ExponentialWith`, `Constant` — constructors
- `AttemptStore` interface + `MemoryStore`
- `Matcher` — error pattern matching
- `sleepCtx` — context-aware sleep

No `slog`. No OTel. No `golang.org/x/sync`. Just Go stdlib.

## Phase 3: Expand the Toolkit

Once the core is extracted, new resilience patterns can be added as sub-modules:

- **Circuit Breaker** — `thumbrise/resilience/circuit`. Track failure rates, trip the circuit, half-open probing. Uses `AttemptStore` for state.
- **Rate Limiter** — `thumbrise/resilience/ratelimit`. Token bucket or sliding window. Composable with Task.
- **Bulkhead** — `thumbrise/resilience/bulkhead`. Concurrency limits per resource. Prevents one slow dependency from consuming all goroutines.
- **Timeout** — already exists inside Task, but could be a standalone decorator.

Each pattern is a separate module with its own `go.mod`. Users pick what they need.

## Non-Goals

- **Not a framework.** No lifecycle management, no DI, no magic. Functions and interfaces.
- **Not a replacement for stdlib.** `context.WithTimeout` is fine. We add value where stdlib doesn't reach.
- **Not feature-complete before extraction.** Extract the core when it's stable. Add patterns incrementally.

## Tracking

- Extraction issue: [#55](https://github.com/thumbrise/autosolve/issues/55)
- Pipeline unification: [#121](https://github.com/thumbrise/autosolve/issues/121) ✅ (done in PR #203)
- Planned features: [`pkg/longrun/TODO.md`](https://github.com/thumbrise/autosolve/blob/main/pkg/longrun/TODO.md)
