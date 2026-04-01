---
title: resilience Package — Overview
description: Composable resilience for function calls — Option/Plugin architecture, Client/CallBuilder API, and extension points.
---

# resilience Package

`pkg/resilience` is a composable resilience toolkit for Go. Zero external dependencies in core. OTEL integration opt-in via sub-package.

::: tip Standalone Package
`resilience` has no dependency on autosolve internals. It lives in `pkg/` and can be used in any Go project.
:::

## Architecture

Two levels of configuration, two extension points:

```
Client (application-wide)          CallBuilder (per-call)
├── Plugin: OTEL metrics           ├── Option: retry
├── Plugin: circuit breaker        ├── Option: timeout
└── Plugin: logging                └── Option: rate limit
```

**Client** — immutable, thread-safe, one per application. Holds Plugins with shared state.

**CallBuilder** — per-call, fresh on every `Call()`. Holds Options with per-call state.

## Quick Start

```go
// Stateless shortcut — no client, no plugins
err := resilience.Do(ctx, fn,
    retry.On(ErrTimeout, 3, backoff.Exponential(1*time.Second, 30*time.Second)),
)

// Client with OTEL plugin
client := resilience.NewClient(rsotel.Plugin())

err := client.Call(fn).
    With(retry.On(ErrTimeout, 3, backoff.Exponential(1*time.Second, 30*time.Second))).
    Do(ctx)
```

## Extension Points

| Type | What | Lifecycle | State |
|------|------|-----------|-------|
| **Option** | `func(ctx, call) error` | Per-call | Fresh each `Do()` |
| **Plugin** | Interface: `Name()` + `Events()` | Client-level | Shared across calls |
| **Preset** | Tested combination of Options | Per-call | Fresh each `Do()` |

Options are the universal extension point. Any resilience pattern — retry, timeout, circuit breaker, bulkhead, hedge — is an Option. Sub-packages provide ready-made Options. Community can publish their own.

Plugins observe all calls via Events hooks without affecting control flow. Use for metrics, logging, circuit breaker state machines.

## Package Layout

```
pkg/resilience/
├── resilience.go       // Option, Plugin, Client, CallBuilder, Do, Events
├── sleepctx.go         // SleepCtx — context-aware sleep
├── backoff/
│   └── backoff.go      // Func, Exponential, Constant, Default
├── retry/
│   └── retry.go        // On, OnFunc, WithWaitHint
└── otel/
    └── retry_hook.go   // Plugin() — OTEL metrics
```

## Chapter Guide

| Page | What you'll learn |
|------|-------------------|
| **[Options & Plugins](./options-plugins)** | The two extension points — how they work, when to use which, how to write your own. |
| **[Retry](./retry)** | `retry.On` / `retry.OnFunc` — error matching, budgets, backoff, WaitHint. |
| **[Backoff](./backoff)** | `backoff.Func` — pure math. Exponential, Constant, custom. |
| **[Observability (OTEL)](./otel)** | `rsotel.Plugin()` — metrics for calls, errors, retries, wait times. |

## Design Principles

- **Option is the universal primitive** — `func(ctx, call) error`. Full control. Any pattern.
- **Plugin observes, Option controls** — two contracts, two lifecycles, no confusion.
- **Per-call state** — Options are fresh on every `Do()`. No shared mutable state. No data races.
- **Events via context** — Plugins attach Events to context. Options extract if needed. No coupling.
- **Backoff is pure math** — `func(attempt int) time.Duration`. Open/closed forever.
- **Zero deps in core** — OTEL, logging, circuit breaker — all in sub-packages.

## Replaces

This package replaces the deleted `pkg/longrun`. See [longrun (deprecated)](../longrun/) for the migration table.
