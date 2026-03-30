---
title: Adding an Integration to autosolve
description: Planned architecture for AI executor integrations in autosolve — how to plug in ra-aid, Ollama, or custom scripts once the dispatch layer ships.
---

# Adding an Integration

::: warning This page is a preview
The AI dispatch / integration layer hasn't been built yet. This page describes the planned architecture and how you'll be able to add new integrations.
:::

## The Plan

autosolve will have an **executor** layer that launches external AI tools when rules match. An integration is a concrete executor — ra-aid, Ollama, a custom script, etc.

## What an Integration Will Look Like

Based on the current architecture patterns, an integration will likely:

1. Implement an `Executor` interface
2. Receive issue/PR context and produce a result
3. Be registered in DI like workers and preflights

```go
// Hypothetical — not implemented yet
type Executor interface {
    Execute(ctx context.Context, task Task) (Result, error)
}
```

## How You Can Help Now

The integration layer is in the design phase. If you have ideas or want to help shape it:

- **Open an issue** with your use case — what AI tool, what triggers, what output format
- **Check the [Ideas & Wishlist](/project/ideas)** — the Rule Engine and Multi-Agent sections are relevant
- **Read the [Idea](/project/idea)** — the original vision describes the executor concept

## Current Extension Points

While the integration layer is being designed, you can already extend autosolve by:

- [Adding a Worker](./adding-worker) — new polling or processing tasks
- Adding infrastructure clients in `internal/infrastructure/` — new API clients, storage backends
- Adding new partition types — see [Schedule Package](/internals/schedule#extension-points)

## Conventions

All code follows the project's [Review Guidelines](https://github.com/thumbrise/autosolve/blob/main/REVIEW.md). Key points for infrastructure code:

- Concrete names — no `Service`, `Manager`, `Handler`
- Rate limiting via HTTP transport (transparent to callers)
- Error interfaces for classification (`Retryable`, `ServicePressure`, `WaitHinted`)
- Tests required for `pkg/` and infrastructure code
