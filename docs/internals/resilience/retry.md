---
title: "resilience: Retry"
description: "retry.On / retry.OnFunc — error matching, budgets, backoff curves, and WaitHint for server-suggested delays."
---

# Retry

`pkg/resilience/retry` provides retry Options for `resilience.Do` and `Client.Call`.

## Basic usage

```go
// Retry on sentinel error — 3 attempts, exponential backoff
retry.On(ErrTimeout, 3, backoff.Exponential(1*time.Second, 30*time.Second))

// Retry on custom classifier — unlimited, constant backoff
retry.OnFunc(isServiceError, retry.Unlimited, backoff.Constant(5*time.Second), "service")
```

## Error matching

`retry.On` supports two forms:

- **Sentinel** — `retry.On(ErrTimeout, ...)` — matches via `errors.Is`
- **Type** — `retry.On((*net.OpError)(nil), ...)` — matches via `errors.As`

`retry.OnFunc` accepts any `func(error) bool` — the escape hatch for custom classification.

## Budget

- `maxRetries > 0` — exact retry count. 3 means 3 retries (4 total calls).
- `retry.Unlimited` (-1) — retries forever until success or context cancellation.
- `maxRetries == 0` — panics. Programmer error.

## WaitHint

Server-suggested wait duration overrides backoff:

```go
retry.OnFunc(isServiceError, retry.Unlimited, backoff.Exponential(5*s, 5*min), "service",
    retry.WithWaitHint(func(err error) time.Duration {
        var wh apierr.WaitHinted
        if errors.As(err, &wh) {
            return wh.WaitDuration()  // e.g. Retry-After: 60s
        }
        return 0  // no hint — use backoff
    }),
)
```

When `WithWaitHint` returns > 0, it overrides `backoff.Func` for that attempt. When it returns 0, normal backoff is used. Backoff remains pure math — WaitHint is the application-level override.

## Multiple retry Options

Multiple retry Options compose as middleware. Each owns its own error matcher and budget:

```go
client.Call(fn).
    With(retry.On(ErrTimeout, 3, backoff.Exponential(1*s, 30*s))).
    With(retry.OnFunc(isServiceError, 5, backoff.Constant(10*s), "service")).
    Do(ctx)
```

Order matters: first option is outermost. If `ErrTimeout` is also a service error, the outermost retry handles it first.

## Observability

Retry emits Events via context helpers. When a Client has plugins (e.g. OTEL), retry automatically emits:

- `OnBeforeCall` / `OnAfterCall` — before and after each attempt
- `OnBeforeWait` — before each backoff sleep, with option name and wait duration
