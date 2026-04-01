---
title: "resilience: Observability (OTEL)"
description: "rsotel.Plugin() — OpenTelemetry metrics for resilience calls, errors, retries, and backoff waits."
---

# Observability (OTEL)

`pkg/resilience/otel` provides an OpenTelemetry Plugin for the resilience package. Opt-in — the core package has zero external dependencies.

## Usage

```go
import rsotel "github.com/thumbrise/autosolve/pkg/resilience/otel"

client := resilience.NewClient(rsotel.Plugin())
```

That's it. All calls through this Client emit OTEL metrics automatically.

## Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `resilience.call.total` | Counter | Every fn call (including retries) |
| `resilience.call.duration_seconds` | Histogram | Duration of each fn call |
| `resilience.call.errors` | Counter | fn calls that returned an error |
| `resilience.retry.total` | Counter | Retry decisions (label: `option`) |
| `resilience.retry.wait_seconds` | Histogram | Backoff wait duration (label: `option`) |

The `option` label carries the retry option name (e.g. `"node"`, `"service"`, `"unregistered"`). This enables per-category dashboards in Grafana.

## How it works

The Plugin implements `resilience.Plugin` — it returns `Events` hooks:

- **OnAfterCall** — increments `call.total`, records `call.duration_seconds`, increments `call.errors` on failure
- **OnBeforeWait** — increments `retry.total`, records `retry.wait_seconds` with the option name label

No `OnBeforeCall` — the Plugin doesn't need it. Events are additive — multiple Plugins can coexist.

## No OTEL SDK? No overhead.

If no OTEL SDK is configured, the default no-op meter produces no-op instruments. Zero allocations, zero overhead. The Plugin is always safe to register.

## Design

OTEL is a Plugin, not an Option. It observes — doesn't control execution. Shared across all calls via Client. This is the canonical example of when to use Plugin vs Option.
