---
title: "longrun: Backoff & Retry State"
description: BackoffFunc as a pure mathematical function, AttemptStore for persistent retry counters, and why algorithms don't belong to systems.
---

# Backoff & Retry State

Two fundamental operations extracted from Task into standalone abstractions.

## BackoffFunc — Algorithm as a Value

Exponential backoff is a mathematical function. Like Fibonacci. Like sorting. It computes a value from an input. It doesn't need a struct. It doesn't need to know about contexts, timers, or tasks.

```go
type BackoffFunc func(attempt int) time.Duration
```

One line. Any function with this signature is a valid backoff strategy.

### Built-in Constructors

```go
// Classic doubling: 2s, 4s, 8s, 16s, ..., capped at 2m
longrun.Exponential(2*time.Second, 2*time.Minute)

// Custom multiplier: 1s, 1.5s, 2.25s, 3.375s, ...
longrun.ExponentialWith(1*time.Second, 30*time.Second, 1.5)

// Fixed delay (useful for retry-after or testing)
longrun.Constant(5*time.Second)

// Sensible default: Exponential(1s, 30s)
longrun.DefaultBackoff()
```

### Custom Backoff

No interface to implement. No adapter. Just a function:

```go
// Jittered backoff — prevents thundering herd
func jittered(attempt int) time.Duration {
    base := time.Second * time.Duration(1<<attempt)
    return base + time.Duration(rand.Int63n(int64(base/2)))
}

// Use it directly
longrun.TransientRule{
    Err: ErrTimeout, MaxRetries: 5, Backoff: jittered,
}
```

### Why Not a Struct?

The old `BackoffConfig` had three responsibilities:
1. **Configuration** — `Initial`, `Max`, `Multiplier` fields
2. **Computation** — `Duration(attempt)` method
3. **IO** — `Wait(ctx, attempt)` method that created a timer and did a select

A struct that configures, computes, AND sleeps is three things pretending to be one. `BackoffFunc` is just the computation. Sleep is `sleepCtx` — a standalone function. Configuration is captured in the closure.

## AttemptStore — Persistent Retry State

Retry counters were the hidden entity. Two implementations lived inside Task:
- `RuleTracker.attempt int` — per-rule, with budget checking
- `map[ErrorCategory]int` — per-category baseline, no budget

Same concept. Different code. Both in-memory only.

```go
type AttemptStore interface {
    Increment(key string) int  // returns value BEFORE increment (0-based)
    Get(key string) int
    Reset()                    // clears all counters (successful tick)
}
```

Keys are opaque strings formed by handlers: `"rule:fetch issues"`, `"baseline:node"`, `"baseline:degraded"`. The store doesn't interpret them.

### **Stable Keys — Safe for Persistent Stores**

**Rule keys are stable across deployments.** Sentinel errors derive their key from the error message automatically. Typed nil pointer errors (`(*net.OpError)(nil)`) require an explicit `Key` field — construction panics without it.

```go
// Sentinel — Key auto-derived from error message "fetch issues"
{Err: ErrFetchIssues, Backoff: longrun.Exponential(2*time.Second, 2*time.Minute)}

// Typed nil pointer — Key MUST be set explicitly
{Err: (*net.OpError)(nil), Key: "net-op", Backoff: longrun.Exponential(1*time.Second, 30*time.Second)}

// Explicit Key — always wins, use for full control
{Err: ErrTimeout, Key: "github-timeout", Backoff: longrun.Exponential(5*time.Second, 5*time.Minute)}
```

**Reordering rules between deployments is safe.** Each rule is identified by its Key, not by its position in the slice. Persistent backoff state maps to the right rule regardless of order.

### Default: MemoryStore

```go
// Zero config — current behavior
task := longrun.NewOneShotTask("migrate", work, rules)
// internally creates MemoryStore
```

`MemoryStore` is exported so users can wrap it with decorators (logging, metrics).

### Persistent Store

```go
// Survive process restarts without losing backoff progress
task := longrun.NewOneShotTask("migrate", work, rules,
    longrun.WithAttemptStore(myRedisStore),
)
```

Why this matters: a worker in degraded mode at attempt 47 (exponential backoff ≈ minutes between retries) restarts. Without persistent state, it starts at attempt 0 — minimum backoff — and hammers the recovering service with rapid retries.

### Budget Checking

`AttemptStore` only counts. Budget enforcement lives in `doRetry` — the shared retry algorithm that both `ruleFailureHandler` and `baselineFailureHandler` delegate to. The store is dumb — `doRetry` is smart. This keeps the interface minimal and the store easily implementable.
