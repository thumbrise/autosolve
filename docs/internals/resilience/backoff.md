---
title: "resilience: Backoff"
description: "backoff.Func — pure mathematical function for retry delays. Exponential, Constant, or write your own."
---

# Backoff

`pkg/resilience/backoff` provides `Func` — a pure function that computes retry delay from attempt index.

## The type

```go
type Func func(attempt int) time.Duration
```

That's it. A function. No interface, no struct, no state. Any `func(int) time.Duration` works.

## Built-in functions

```go
// Classic doubling: 1s, 2s, 4s, 8s, ..., capped at 30s
backoff.Exponential(1*time.Second, 30*time.Second)

// Custom multiplier: 1s, 1.5s, 2.25s, ..., capped at 30s
backoff.ExponentialWith(1*time.Second, 30*time.Second, 1.5)

// Always the same: 5s, 5s, 5s, ...
backoff.Constant(5*time.Second)

// Sensible default: Exponential(1s, 30s)
backoff.Default()
```

## Custom backoff

Write any function:

```go
// Linear: 1s, 2s, 3s, 4s, ...
func linear(attempt int) time.Duration {
    return time.Duration(attempt+1) * time.Second
}

// Jittered exponential
func jittered(attempt int) time.Duration {
    base := time.Second * time.Duration(1<<attempt)
    jitter := time.Duration(rand.Int63n(int64(base / 2)))
    return base + jitter
}
```

## Design

Backoff is **open/closed forever**. The type is `func(int) time.Duration`. You can't break it. You can't need to extend it. Fibonacci, polynomial, decorrelated jitter — write the math, pass the function.

WaitHint (server-suggested delay like Retry-After) is **not** backoff. It's an override at the retry level via `retry.WithWaitHint`. Backoff stays pure math.
