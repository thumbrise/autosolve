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

When the API is stable, extract into a standalone multi-module repository.

### The Foundation: `resilience.Do`

The extraction introduces a new primitive — `Do` — a single resilient call that blocks until success, budget exhaustion, or context cancellation:

```go
err := resilience.Do(ctx, func(ctx context.Context) error {
    return client.CreateWebhook(ctx, payload)
},
    retry.On(ErrTimeout, 5, backoff.Exponential(1*time.Second, 30*time.Second)),
    circuit.Breaker("webhooks", circuit.Threshold(5)),
    rsotel.Trace("webhook.create"),
)
```

Every resilience pattern is an `Option` — a middleware that wraps the call. Options compose like Lego. Add a line — pattern appears. Remove a line — pattern disappears. Order matters: options read top-to-bottom as an execution pipeline.

`Do` is the foundation. `Task` is `Do` in a loop. `Runner` is N `Task` in an errgroup. Three primitives, one stack.

### Repository Layout

```
thumbrise/resilience/
├── go.mod                  // core: Do, Option, Compose, BackoffFunc
├── backoff/                // Exponential, Constant, ExponentialWith
├── retry/                  // retry.On, AttemptStore, MemoryStore, Matcher
├── task/                   // Task, Runner, failureHandler pipeline
├── circuit/                // circuit.Breaker — state machine
│   └── go.mod
├── bulkhead/               // bulkhead.Max — concurrency limiter
├── hedge/                  // hedge.After — speculative parallel call
├── shed/                   // shed.OnLatency — load shedding
├── fallback/               // fallback.To — fallback on exhaustion
├── timeout/                // timeout.After — per-call deadline
├── ratelimit/              // ratelimit.Wait — token bucket
│   └── go.mod
├── preset/                 // preset.HTTP, preset.SQL — common transient sets
├── otel/                   // rsotel.Trace — automatic span per Do
│   └── go.mod
└── grpc/                   // preset.GRPC — gRPC transient errors
    └── go.mod
```

### Why Multi-Module?

Go modules work at the `go.mod` level. A single `go.mod` means `go mod tidy` downloads everything — even packages you don't use. Their transitive dependencies land in your `go.sum`.

Multi-module (like `go.opentelemetry.io/otel`) isolates heavy dependencies:

- `go get thumbrise/resilience/retry` → pulls core + retry. No OTel SDK.
- `go get thumbrise/resilience/otel` → pulls core + OTel bindings. Only if you want tracing.
- `go get thumbrise/resilience/grpc` → pulls core + gRPC codes. Only if you use gRPC.

Users import exactly what they need. No bloat.

### Core Module: Zero Dependencies

The core (`thumbrise/resilience`) would contain only fundamental abstractions:

- `Do` — single resilient call, the foundation primitive
- `Option` / `Compose` — middleware composition
- `BackoffFunc` — pure function type
- `Exponential`, `ExponentialWith`, `Constant` — constructors
- `AttemptStore` interface + `MemoryStore`
- `Matcher` — error pattern matching
- `sleepCtx` — context-aware sleep

No `slog`. No OTel. No `golang.org/x/sync`. Just Go stdlib.

## Phase 3: Resilience Patterns

Once the core is extracted, patterns are added as sub-modules. Each is a standalone Lego brick.

### Retry

Already exists in longrun. Extracted as `retry.On` — per-error matching with independent budgets and backoff curves.

### Circuit Breaker

`thumbrise/resilience/circuit`. Track failure rates, trip the circuit, half-open probing. When open — `Do` returns `circuit.ErrOpen` immediately, no retry attempted.

### Bulkhead

`thumbrise/resilience/bulkhead`. Semaphore-based concurrency limit per resource. Prevents one slow dependency from consuming all goroutines. Without it: S3 slows down → 500 goroutines hang → OOM.

### Hedge

`thumbrise/resilience/hedge`. Speculative parallel request. If the first call doesn't respond within a threshold, fire a second in parallel. First response wins. Google uses this in Bigtable — tail latency p99 drops dramatically.

### Load Shedding

`thumbrise/resilience/shed`. Reject new calls when the system is overloaded. Unlike bulkhead (concurrency cap), shed watches latency or queue depth and starts refusing before degradation hits.

### Fallback

`thumbrise/resilience/fallback`. When retry budget is exhausted, call an alternative instead of returning an error. The fallback function is `Do`-compatible — nest your own retry inside it.

### Rate Limiter

`thumbrise/resilience/ratelimit`. Token bucket or sliding window. `ratelimit.Wait` blocks until a token is available. Composable with any other pattern.

### Timeout

`thumbrise/resilience/timeout`. Per-call deadline via `context.WithTimeout`. Already exists inside Task, extracted as a standalone option.

## Phase 4: Presets

Common transient error sets packaged as ready-made `Option` bundles. A preset is just `Compose(...)` — no magic.

```go
// Standard HTTP transients: timeout, DNS, connection refused, 502/503/429
err := resilience.Do(ctx, callAPI, preset.HTTP())

// Standard gRPC transients: Unavailable, DeadlineExceeded, ResourceExhausted
err := resilience.Do(ctx, callGRPC, preset.GRPC())

// Standard SQL transients: connection lost, deadlock, lock timeout
err := resilience.Do(ctx, execQuery, preset.SQL())
```

Presets compose with custom options — custom `retry.On` for a specific error overrides the preset's default for that error.

Presets with no external dependencies live in `preset/`. Protocol-specific presets (`preset.GRPC`) live in their own module with a separate `go.mod`.

## Non-Goals

- **Not a framework.** No lifecycle management, no DI, no magic. Functions and interfaces.
- **Not a replacement for stdlib.** `context.WithTimeout` is fine. We add value where stdlib doesn't reach.
- **Not feature-complete before extraction.** Extract the core when it's stable. Add patterns incrementally.

## Tracking

- Extraction issue: [#55](https://github.com/thumbrise/autosolve/issues/55)
- Pipeline unification: [#121](https://github.com/thumbrise/autosolve/issues/121) ✅ (done in PR #208)
