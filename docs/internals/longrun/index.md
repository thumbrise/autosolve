---
title: longrun Package — Overview
description: Standalone Go package for long-running tasks with per-error retry, exponential backoff, degraded mode, and built-in OpenTelemetry spans.
---

# longrun Package

`pkg/longrun` is a self-contained Go package for long-running tasks. Zero internal dependencies beyond `golang.org/x/sync`. Designed for extraction into a standalone resilience toolkit once the API stabilizes (see [#55](https://github.com/thumbrise/autosolve/issues/55)).

::: tip Standalone Package
`longrun` has no dependency on autosolve internals. It lives in `pkg/` and can be used in any Go project.
:::

## Two Primitives

- **Task** — one-shot or interval, with optional per-error retry and backoff. Self-contained — works standalone via `task.Wait(ctx)` or managed by Runner.
- **Runner** — orchestrates N tasks. Cancels all on permanent failure. LIFO shutdown hooks. Injects Baseline policies silently.

## Execution Model

```
Task.Wait(ctx)
  └→ restartLoop (restart loop + backoff)
       └→ runLoop (ticker or one-shot)
            └→ runOnce (single invocation ± timeout)
                 └→ automatic OTEL span
```

Task is the root of composition for a single unit of work. Runner coordinates multiple Tasks but delegates execution entirely.

## Quick Start

```go
// One-shot task (e.g. migration)
task := longrun.NewOneShotTask("migrate", db.AutoMigrate, nil)

// Interval task with per-error retry
task := longrun.NewIntervalTask("poll", 10*time.Second, poller.Run, []longrun.TransientRule{
    {Err: ErrFetchIssues, MaxRetries: 5, Backoff: longrun.Exponential(2*time.Second, 60*time.Second)},
    {Err: ErrStoreIssues, MaxRetries: longrun.UnlimitedRetries, Backoff: longrun.Exponential(100*time.Millisecond, 2*time.Second)},
})

// Runner with Baseline — invisible safety net for all tasks
runner := longrun.NewRunner(longrun.RunnerOptions{
    Logger: logger,
    Baseline: longrun.NewBaselineDegraded(
        longrun.Policy{Backoff: longrun.Exponential(2*time.Second, 2*time.Minute)},   // Node
        longrun.Policy{Backoff: longrun.Exponential(5*time.Second, 5*time.Minute)},   // Service
        longrun.Policy{Backoff: longrun.Exponential(30*time.Second, 5*time.Minute)},  // Default (degraded)
        myClassifier,
    ),
})
runner.Add(task)
err := runner.Wait(ctx)
```

## Chapter Guide

This is a multi-page chapter. Start here for the overview, then dive into specifics:

| Page | What you'll learn |
|---|---|
| **[Failure Pipeline](./pipeline)** | How errors flow through handlers: TransientRules, Baseline classification, degraded mode. The unified `failureHandler` interface. |
| **[Backoff & Retry State](./backoff)** | `BackoffFunc` as a pure function. `AttemptStore` for persistent retry state. Why algorithms don't belong to systems. |
| **[Observability](./observability)** | Automatic OTEL spans, baseline retry metrics, degraded mode alerting. |

## Design Principles

- **Unified failure pipeline** — TransientRules and Baseline are both `failureHandler` implementations. One loop, no branching.
- **BackoffFunc is a pure function** — `(attempt) → duration`. Any `func(int) time.Duration` works. Open/Closed at maximum.
- **AttemptStore** — retry counters behind an interface. Default: in-memory. Plug Redis/SQLite for persistence.
- **Transient errors whitelist** — empty handlers = all errors permanent. You must explicitly opt in to retry.
- **Signals are not the package's job** — Runner takes a `ctx`, caller handles `signal.NotifyContext`.
- **LIFO shutdown** — last added task shuts down first, like `defer`.

## Links

- Planned features: [`pkg/longrun/TODO.md`](https://github.com/thumbrise/autosolve/blob/main/pkg/longrun/TODO.md)
- Extraction roadmap: [#55](https://github.com/thumbrise/autosolve/issues/55)
