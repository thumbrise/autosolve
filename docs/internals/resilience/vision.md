---
title: "resilience: Vision"
description: "Where the resilience package is headed — Presets, validation, new patterns, plugin toolkit, and introspection."
---

# Vision

This page captures the direction, not the plan. No deadlines. No promises. When a real use case demands it — we build it.

## Presets — tested recipes

Options compose freely. Too freely — footguns are real. Presets are tested combinations with metadata:

```go
client.Call(fn).
    WithPreset(preset.ResilientHTTP("github")).
    With(retry.On(mySpecificErr, 5, bo)).  // custom on top
    Do(ctx)
```

Preset is not a bag of Options. It's a **recipe** — ingredients + order + tested compatibility + name. Core can introspect presets, validate conflicts, log what was applied.

## Core validation

Two presets with conflicting retry rules? Timeout shorter than backoff max? Core detects at `Do()` time:

```
resilience: conflict detected
  preset "ResilientHTTP" → retry.On(ErrTimeout, 3, exponential)
  preset "SmartRetry"    → retry.On(ErrTimeout, 5, constant)
  two retry options match the same error
```

Fail fast. Not at 3am in production.

## New patterns as Options

Every pattern is `func(ctx, call) error`. Future sub-packages:

- **timeout** — `timeout.After(5*time.Second)` — per-call deadline
- **circuit** — `circuit.Breaker("github")` — shared state via Plugin, per-call check via Option
- **bulkhead** — `bulkhead.Max(20)` — concurrency semaphore
- **hedge** — `hedge.After(100*time.Millisecond)` — speculative parallel call
- **ratelimit** — `ratelimit.Wait(limiter)` — token bucket before call
- **fallback** — `fallback.To(backupFn)` — alternative on failure

Each is an independent package. Zero coupling between patterns. Community can publish their own.

## Plugin toolkit

Simple plugins shouldn't need to implement `func(ctx, call) error` from scratch:

```go
// Future: plugin helpers for common patterns
plugin.BeforeCall(func(ctx context.Context) (context.Context, error) { ... })
plugin.AfterCall(func(ctx context.Context, err error) { ... })
```

Three layers of comfort: userland uses ready Options, plugin authors use helpers, advanced authors use raw `func(ctx, call) error`.

## Introspection and tooling

`resilience.Dump(client, opts...)` — describe the pipeline before running:

```
Pipeline:
  1. [preset:ResilientHTTP] timeout(10s) → retry(ErrTimeout, 3, exp) → retry(Err5xx, 3, exp)
  2. [option] retry(ErrRateLimit, 5, const(60s), hint:serviceWaitHint)
  3. [plugin:otel] OnAfterCall, OnBeforeWait
```

Future: linter that catches footguns statically. Graph visualization for dashboards.

## What we won't do

- **Config file for resilience** — resilience is code, not YAML. Patterns are engineering decisions.
- **Global state** — no package-level defaults, no init(). Client is explicit.
- **Magic** — no auto-detection of error types, no implicit retry. Every behavior is opt-in.
