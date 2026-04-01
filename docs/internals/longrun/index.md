---
title: longrun Package (Deprecated)
description: "pkg/longrun has been replaced by pkg/resilience and the schedule package."
---

# longrun Package (Deprecated)

::: danger Deprecated
`pkg/longrun` has been deleted. All functionality has been replaced by:

- **[`pkg/resilience`](../resilience/)** — composable resilience for function calls (retry, backoff, plugins)
- **[Schedule package](../schedule)** — scheduler owns the execution loop directly

See [Devlog #13](../../devlog/013-killing-longrun) for the full story of this transformation.
:::

## What was longrun?

longrun was a task orchestration package: `Task`, `Runner`, `Baseline`, `Policy`, `TransientRule`, `failureHandler`, `AttemptStore`. ~1500 lines wrapping `errgroup` + `time.Ticker` + retry logic.

## Why was it killed?

It was `make_u32_from_two_u16()` — an abstraction that made the world worse, not better. Every concept it introduced (Baseline, Policy, ErrorCategory, ClassifierFunc, degraded mode) collapsed into `retry.OnFunc` with different parameters. The scheduler needed 5 minutes of `errgroup` + `ticker`, not a framework.

## Where did it go?

| longrun concept | Replaced by |
|---|---|
| `Task` + `Runner` | Scheduler's `runOnce` / `runLoop` + `errgroup` |
| `TransientRule` | `retry.On` / `retry.OnFunc` ([resilience.Option](../resilience/retry)) |
| `Baseline` + `Policy` | `strictRetryOptions` / `resilientRetryOptions` in schedule |
| `ErrorCategory` + `ClassifierFunc` | `isNodeError` / `isServiceError` in schedule |
| `BackoffFunc` | `backoff.Func` ([resilience/backoff](../resilience/backoff)) — unchanged |
| `AttemptStore` | Attempt counter inside Option closure — per-call, no shared state |
| OTEL metrics | `rsotel.Plugin()` ([resilience/otel](../resilience/otel)) |
| Degraded mode | Catch-all `retry.OnFunc(always, ...)` with name `"unregistered"` |

## Historical docs

The sub-pages (pipeline, backoff, observability, roadmap) have been removed from navigation. They remain in git history for reference.
