---
title: autosolve Configuration Reference
description: Complete config.yml reference for autosolve — GitHub tokens, multi-repo setup, rate limiting, OpenTelemetry, and environment variable overrides.
---

# Configuration

autosolve uses a single `config.yml` file. Every field can be overridden via environment variables with the `AUTOSOLVE_` prefix (e.g. `github.token` → `AUTOSOLVE_GITHUB_TOKEN`).

## Minimal Config

```yaml
github:
  token: ghp_your_token_here
  repositories:
    - owner: your-org
      name: your-repo
```

This is enough to start. Everything else has sensible defaults.

## Full Reference

```yaml
log:
  debug: true          # verbose logging (default: false)
  source: false        # include source file in logs (default: false)

database:
  sqlitePath: runtime/database/sqlite.db   # SQLite file path

github:
  token: ghp_...                # GitHub personal access token
  repositories:                 # list of repos to monitor
    - owner: thumbrise
      name: autosolve
    - owner: thumbrise
      name: otelext
  httpClientTimeout: 5s         # HTTP client timeout
  rateLimit:
    minInterval: 1s             # minimum interval between GitHub API calls
  issues:
    parseInterval: 5s           # how often to poll for new issues

otel:                           # OpenTelemetry (disabled by default)
  sdkDisabled: true             # set to false to enable
  serviceName: autosolve
  resourceAttributes: "service.version=1.0.0,deployment.environment=production"
  propagators: "tracecontext,baggage"
  traces:
    exporter: otlp
    sampler: parentbased_always_on
    samplerArg: "1.0"
  metrics:
    exporter: otlp
  logs:
    exporter: otlp
  exporter:
    endpoint: "localhost:4317"
    protocol: grpc
    headers: "Authorization=token"
    timeout: 30s
```

## Multiple Repositories

Just add more entries under `repositories`. Each repo becomes an independent partition — its own preflight validation, its own polling cursor, its own state.

```yaml
github:
  repositories:
    - owner: your-org
      name: repo-one
    - owner: your-org
      name: repo-two
    - owner: another-org
      name: repo-three
```

## Environment Variable Override

Any config field can be set via env var. Nested fields use underscores:

```bash
export AUTOSOLVE_GITHUB_TOKEN=ghp_xxx
export AUTOSOLVE_DATABASE_SQLITEPATH=/var/data/autosolve.db
export AUTOSOLVE_OTEL_SDKDISABLED=false
```

## See Also

- [Observability](./observability) — detailed OTEL setup
- [Getting Started](./getting-started) — quick start guide
