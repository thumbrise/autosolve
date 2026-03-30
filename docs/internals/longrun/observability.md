---
title: "longrun: Observability"
description: Automatic OTEL spans, baseline retry metrics, and degraded mode alerting in longrun.
---

# Observability

longrun provides observability out of the box. No SDK configured → zero overhead. SDK configured → full visibility.

## Automatic Tracing

Every invocation of the work function is wrapped in an OpenTelemetry span inside `runOnce`. The span is named after the task.

```text
[longrun/task: "polling issues"]           ← automatic span from longrun
  └─[IssuePolling.work]                   ← user's child span (optional)
    └─[Parser.Run]                         ← domain span (optional)
      └─[SQL INSERT]                       ← infra span (optional)
```

On error: `span.RecordError(err)` + `span.SetStatus(codes.Error, ...)`. Users get full observability without writing any OTEL code in their work functions.

Combined with a `slog.Handler` that extracts span context (`trace_id`, `span_id`), every log line emitted via `logger.InfoContext(ctx, ...)` is automatically correlated with the active trace.

## Baseline Retry Metrics

The `baselineFailureHandler` emits OTel metrics on every retry:

| Metric | Type | Labels | Purpose |
|--------|------|--------|---------|
| `longrun_baseline_retry_total` | Counter | `task`, `category` | Each retry via baseline (node/service/degraded) |
| `longrun_degraded_total` | Counter | `task` | Each retry in degraded mode |
| `longrun_degraded_duration_seconds` | Histogram | `task` | Time spent in a single degraded wait |

### Alerting

These metrics enable practical alerts:

- **"task X in degraded for 10 minutes"** — `longrun_degraded_duration_seconds` histogram
- **"baseline retries spiking for category node"** — `longrun_baseline_retry_total` rate
- **"task entered degraded mode"** — `longrun_degraded_total` counter increment

## Structured Logging

All retry events are logged with structured fields:

```
INFO  transient error, retrying    task=poll attempt=3 error="connection refused" backoff=8s
ERROR DEGRADED: unknown error      task=poll error="unexpected status 418"
ERROR max retries reached          task=poll error="connection refused" max_retries=5
```

Every log call uses `logger.InfoContext(ctx, ...)` — trace context is always propagated.
