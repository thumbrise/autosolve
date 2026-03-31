---
title: autosolve Error Handling & Retry
description: Layered error handling in autosolve — TransientRules, Baseline classifier, degraded mode, and per-error exponential backoff explained.
---

# Error Handling & Retry

autosolve has a layered error handling system. Domain code declares errors, infrastructure classifies them, and `longrun` decides what to do.

## The Pipeline

```
err from work()
  │
  ├─ [1] TransientRules (per-task, explicit)
  │       Match by sentinel (errors.Is) or type (errors.As)
  │       Own retry budget + backoff per rule
  │
  ├─ [2] Baseline classifier (Runner-level)
  │   ├─ Built-in: net.OpError, DNS, timeout, EOF → Node
  │   ├─ User: apierr.WaitHinted → Service (with explicit wait)
  │   ├─ User: apierr.ServicePressure → Service
  │   ├─ User: apierr.Retryable → Service
  │   └─ Not classified → Unknown
  │
  └─ [3] Unknown error handling
         Degraded policy set → retry internally, log ERROR
         Degraded policy nil → permanent error → crash
```

## Two Layers of Retry

### TransientRules (Task-Level)

Explicit per-error retry. The task author decides which errors are retryable and how aggressively:

```go
rules := []longrun.TransientRule{
    {Err: ErrFetchIssues, MaxRetries: 5, Backoff: longrun.Exponential(2*time.Second, 60*time.Second)},
    {Err: ErrStoreIssues, MaxRetries: longrun.UnlimitedRetries, Backoff: longrun.Exponential(100*time.Millisecond, 2*time.Second)},
}
```

Different errors → different budgets. GitHub API under load? Retry carefully. Local SQLite? Retry aggressively.

### Baseline (Runner-Level)

Invisible protection configured once on Runner. Tasks don't know it exists. Each `Policy` has an optional `Retries` budget: `0` (zero-value) means unlimited, `>0` means exact count.

| Category | Meaning | Policy |
|----------|---------|--------|
| **Node** | Transport failure (TCP, DNS, TLS) | Aggressive retry — network recovers |
| **Service** | Remote pressure (rate limit, 5xx) | Gentle retry — don't pile on |
| **Unknown** | Not classified by anyone | Degraded (if set) or permanent |

Baseline runs *after* TransientRules. If no rule matches, Baseline classifies.

## Sentinel Errors at Domain Boundaries

Domain code defines sentinel errors where the problem is understood:

```go
var (
    ErrFetchIssues = errors.New("fetch issues")
    ErrStoreIssues = errors.New("store issues")
    ErrReadCursor  = errors.New("read cursor")
)
```

Errors are wrapped with sentinels at boundaries: `fmt.Errorf("%w: %w", ErrFetchIssues, err)`. First `%w` is the domain sentinel (catchable via `errors.Is`), second preserves the original chain.

## Degraded Mode

When a worker gets an unknown error and Degraded policy is set:
- Retries internally with Degraded backoff (30s → 5min)
- When `Retries` is `0` (unlimited, the default) — **never returns the error to Runner**. Like `docker restart: always`
- When `Retries > 0` — retries up to the budget, then returns a permanent error to Runner
- Logs at ERROR level on every retry

When Degraded is nil (preflights), unknown errors are permanent — crash early.

## WaitDuration Override

When the classifier returns `ErrorClass.WaitDuration > 0` (e.g. from `Retry-After` header on HTTP 429), the task sleeps exactly that duration instead of using exponential backoff.

## Source

- [`pkg/longrun/`](https://github.com/thumbrise/autosolve/tree/main/pkg/longrun) — the full retry engine
- [`internal/application/planner.go`](https://github.com/thumbrise/autosolve/blob/main/internal/application/planner.go) — `InfraClassifier()` implementation
- [`internal/contracts/apierr/`](https://github.com/thumbrise/autosolve/tree/main/internal/contracts/apierr) — error interfaces
