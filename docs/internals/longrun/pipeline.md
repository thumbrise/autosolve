---
title: "longrun: Failure Pipeline"
description: How longrun routes errors through a unified handler chain — TransientRules, Baseline classification, and degraded mode.
---

# Failure Pipeline

When `work()` returns an error, Task doesn't decide what to do. It asks its handlers.

## The Loop

```go
for _, h := range t.handlers {
    result := h.Handle(ctx, err)
    if result != errSkip {
        return result // nil = retry, error = permanent
    }
}
return err // no handler claimed it → permanent
```

Each handler returns one of three things:
- **`errSkip`** — "not my error, ask the next handler"
- **`nil`** — "I handled it, retry the work function"
- **an error** — "permanent failure, stop the task"

Handlers are ordered. Rules first, Baseline last. First match wins.

## The Retry Algorithm — `doRetry`

Both `ruleFailureHandler` and `baselineFailureHandler` delegate the actual retry to a single internal function `doRetry` in `retry.go`. The skeleton is: increment attempt → check budget → compute wait duration → log → sleep → return nil. Handlers own matching and metrics — `doRetry` owns the retry mechanics.

## TransientRules — Explicit Error Matching

Each `TransientRule` becomes a `ruleFailureHandler`. It matches errors via `errors.Is` or `errors.As`, then delegates to `doRetry` with its own budget and `BackoffFunc`.

```go
task := longrun.NewIntervalTask("poll", 10*time.Second, poller.Run, []longrun.TransientRule{
    {Err: ErrFetchIssues, MaxRetries: 5, Backoff: longrun.Exponential(2*time.Second, 60*time.Second)},
    {Err: ErrStoreIssues, MaxRetries: longrun.UnlimitedRetries, Backoff: longrun.Exponential(100*time.Millisecond, 2*time.Second)},
})
```

Different errors get different budgets. GitHub API rate limit → careful retry. Local DB write error → aggressive retry. The key insight: **how retryable is this specific error?**

### MaxRetries Semantics

| Value | Meaning |
|-------|---------|
| `0` (zero-value) | `DefaultMaxRetries` (3) — safe default, logs a warning |
| `-1` (`UnlimitedRetries`) | No limit — explicit opt-in |
| `> 0` | Exact retry count |

When an interval task completes a successful tick, all attempt counters reset. Intermittent failures separated by successful ticks never accumulate.

## Baseline — Invisible Safety Net

Runner injects a `baselineFailureHandler` into every task at `Add()` time. Tasks don't know it's there. After classification, the handler delegates to `doRetry` with the selected policy's budget and backoff.

Each `Policy` has a `Retries` field: `0` (zero-value) means unlimited retries, `>0` means exact budget. This is different from `TransientRule.MaxRetries` where `0` means `DefaultMaxRetries(3)`. The conversion is handled by `resolveBaselineMaxRetries`.

Baseline classifies errors through a three-step pipeline:

```
err from work()
  │
  ├─ [1] Built-in transport classify (net.OpError, DNS, timeout, EOF → Node)
  ├─ [2] User classifier via Baseline.Classify (apierr interfaces → Service)
  └─ [3] Not classified → Unknown
         Default policy set → retry with loud ERROR log (degraded mode)
         Default policy nil → errSkip (let pipeline return permanent)
```

### Error Categories

| Category | Meaning | Typical Policy |
|----------|---------|----------------|
| **Node** | Transport failure (TCP, DNS, TLS, timeout) | Aggressive — network will recover |
| **Service** | Remote pressure (rate limit, 5xx, maintenance) | Gentle — don't kick them while they're down |
| **Unknown** | Not recognized by any classifier | Default policy or permanent |

Users can define custom categories: `const CategoryDatabase longrun.ErrorCategory = 10`

### Wait Duration Override

When `ClassifierFunc` returns `ErrorClass.WaitDuration > 0`, the handler sleeps exactly that duration instead of calling `BackoffFunc`. Typical source: `Retry-After` header on HTTP 429.

## Degraded Mode

When a task gets an unknown error and Baseline has a Default policy:

- Retries internally with the Default policy's backoff
- When `Default.Retries` is `0` (unlimited, the default) — never bubbles up to Runner. Like Docker `restart: always`
- When `Default.Retries > 0` — retries up to the budget, then returns a permanent error to Runner
- Logs at `ERROR` level on every retry
- Emits `longrun_degraded_total` and `longrun_degraded_duration_seconds` metrics

When Default is nil (e.g. preflights): unknown errors are permanent. Crash early, fix your config.

## How Runner Builds the Pipeline

```go
// User creates task with rules
task := longrun.NewOneShotTask("migrate", work, rules)
// handlers = [ruleHandler0, ruleHandler1, ...]

// Runner appends baseline
runner.Add(task)
// handlers = [ruleHandler0, ruleHandler1, ..., baselineHandler]
```

Priority is preserved: user rules always run first. Baseline is the fallback.
