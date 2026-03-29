---
title: autosolve longrun Package
description: Standalone Go package for long-running tasks with per-error retry, exponential backoff, degraded mode, and built-in OpenTelemetry spans.
---

# longrun Package

`pkg/longrun` is a self-contained Go package for long-running tasks. Zero internal dependencies beyond `golang.org/x/sync`. Designed for extraction into a standalone module once the API stabilizes.

::: tip Standalone Package
`longrun` has no dependency on autosolve internals. It lives in `pkg/` and can be used in any Go project. The plan is to extract it into its own repo after battle-testing.
:::

## What It Does

Two primitives:
- **Task** — one-shot or interval, with optional per-error retry and backoff
- **Runner** — orchestrates N tasks, cancels all on permanent failure, LIFO shutdown

## Task Execution Model

```
Task.Wait(ctx)
  └→ runWithPolicy (restart loop + backoff)
       └→ runLoop (ticker or one-shot)
            └→ runOnce (single invocation ± timeout)
                 └→ automatic OTEL span
```

## Quick Examples

```go
// One-shot task (e.g. migration)
task := longrun.NewOneShotTask("migrate", db.AutoMigrate, nil)

// Interval task with per-error retry
task := longrun.NewIntervalTask("poll", 10*time.Second, poller.Run, []longrun.TransientRule{
    {Err: ErrFetchIssues, MaxRetries: 5, Backoff: longrun.Backoff(2*time.Second, 60*time.Second)},
    {Err: ErrStoreIssues, MaxRetries: longrun.UnlimitedRetries, Backoff: longrun.Backoff(100*time.Millisecond, 2*time.Second)},
})

// Runner with Baseline
runner := longrun.NewRunner(longrun.RunnerOptions{
    Logger: logger,
    Baseline: longrun.Baseline{
        Node:     longrun.Policy{Backoff: longrun.Backoff(2*time.Second, 2*time.Minute)},
        Service:  longrun.Policy{Backoff: longrun.Backoff(5*time.Second, 5*time.Minute)},
        Degraded: &longrun.Policy{Backoff: longrun.Backoff(30*time.Second, 5*time.Minute)},
        Classify: myClassifier,
    },
})
runner.Add(task)
err := runner.Wait(ctx)
```

## Key Concepts

### TransientRules — Per-Error Retry

Each rule binds an error to retry settings. Different errors get different budgets:

```go
type TransientRule struct {
    Err        any           // error sentinel (errors.Is) or *T (errors.As)
    MaxRetries int           // 0 = default (3), -1 = unlimited
    Backoff    BackoffConfig
}
```

Consecutive failures per rule. Successful tick resets all counters.

### Baseline — Invisible Safety Net

Runner-level policies applied to every task. Tasks don't know about Baseline.

Three error categories: **Node** (transport), **Service** (remote pressure), **Unknown** (unclassified). Unknown errors go to Degraded policy if set, or become permanent.

### Degraded Mode

Task-level, not Runner-level. Unknown error + Degraded policy → retry internally, never bubble up to Runner. Logs ERROR on every retry. Like `docker restart: always`.

## Observability

Every `runOnce` invocation is wrapped in an OTEL span. No SDK → no-op tracer, zero overhead. With SDK → every invocation, retry, and error is visible.

Metrics:

| Metric | Type |
|--------|------|
| `longrun_baseline_retry_total` | Counter (per task + category) |
| `longrun_degraded_total` | Counter (per task) |
| `longrun_degraded_duration_seconds` | Histogram (per task) |

## Full Documentation

The package has its own comprehensive README: [`pkg/longrun/README.md`](https://github.com/thumbrise/autosolve/blob/main/pkg/longrun/README.md)

Planned features: [`pkg/longrun/TODO.md`](https://github.com/thumbrise/autosolve/blob/main/pkg/longrun/TODO.md)
