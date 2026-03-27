# Observability

autosolve has OpenTelemetry baked in at every level. Disabled by default — zero overhead until you need it.

## Enable OTEL

Set `otel.sdkDisabled: false` in `config.yml` and point to your collector:

```yaml
otel:
  sdkDisabled: false
  serviceName: autosolve
  exporter:
    endpoint: "localhost:4317"
    protocol: grpc
```

That's it. Traces, metrics, and logs will flow via OTLP/gRPC.

## What Gets Instrumented

### Traces

Every task invocation in `longrun` is automatically wrapped in a span. You get a trace for each polling cycle without writing any OTEL code:

```
[longrun/task: "worker:issue-poller:thumbrise/autosolve"]
  └─ [GitHub API call]
       └─ [SQL INSERT]
```

Errors are recorded on spans automatically: `span.RecordError(err)` + `span.SetStatus(codes.Error, ...)`.

### Metrics

Baseline retries emit counters and histograms:

| Metric | Type | What it tells you |
|--------|------|-------------------|
| `longrun_baseline_retry_total` | Counter | Retries per task and error category |
| `longrun_degraded_total` | Counter | How often a task enters degraded mode |
| `longrun_degraded_duration_seconds` | Histogram | Time spent in degraded state |

Useful alert: *"task X in degraded for 10 minutes"*.

### Logs

All logging uses `slog` with context. When OTEL is configured, every log line carries `trace_id` and `span_id` — automatic correlation with traces, zero boilerplate in business code.

## Compatible Backends

Anything that speaks OTLP: Grafana Tempo, Jaeger, Uptrace, Datadog, Honeycomb, SigNoz, etc.

## OTEL Environment Variables

All config fields can be overridden via env vars:

```bash
export AUTOSOLVE_OTEL_SDKDISABLED=false
export AUTOSOLVE_OTEL_EXPORTER_ENDPOINT=localhost:4317
```

See the [OpenTelemetry SDK env var spec](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/) for the full list.
