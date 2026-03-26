# autosolve

[![CI](https://github.com/thumbrise/autosolve/actions/workflows/ci.yml/badge.svg)](https://github.com/thumbrise/autosolve/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/thumbrise/autosolve.svg)](https://pkg.go.dev/github.com/thumbrise/autosolve)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](/LICENSE)

Available languages: [English](README.md), [Русский](README.ru.md)

Lightweight daemon that polls GitHub repositories and dispatches AI agents to solve issues automatically.

> Original project vision: [docs/IDEA.md](docs/IDEA.md)

## Navigation

- [Review Guidelines](REVIEW.md) — code style, architecture, git conventions
- [Documentation](docs) — additional docs and translations

## Tech Stack

| Component     | Technology                                                                        |
|---------------|-----------------------------------------------------------------------------------|
| Language      | Go                                                                                |
| Database      | SQLite (pure Go, WAL mode)                                                        |
| Migrations    | [goose](https://github.com/pressly/goose)                                         |
| SQL codegen   | [sqlc](https://sqlc.dev)                                                          |
| DI            | [Wire](https://github.com/google/wire)                                            |
| Observability | [OpenTelemetry](https://opentelemetry.io) (traces, metrics, logs → OTLP/gRPC)     |
| CLI           | [cobra](https://github.com/spf13/cobra) + [viper](https://github.com/spf13/viper) |

## Quick Start

### Prerequisites

- Go 1.26+
- [Task](https://taskfile.dev) (optional, but recommended)

### Setup

```bash
git clone https://github.com/thumbrise/autosolve.git
cd autosolve
go mod download
cp config.yml.example config.yml
```

Edit `config.yml` — at minimum set your GitHub token and target repositories:

```yaml
github:
  token: ghp_your_token_here
  repositories:
    - owner: your-org
      name: your-repo
```
<details>
  <summary>OpenTelemetry</summary>

**Disabled by default.**

You can collect [OpenTelemetry](https://opentelemetry.io) data via configuration variables with standard OTEL semantic:

```yaml
otel:
  sdkDisabled: true
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
    headers: "uptrace-dsn=http://aiji-qvjRjFBnObLuzAkpA@localhost:14318?grpc=14317"
    timeout: 10s
```

All config fields can be overridden via environment variables with `AUTOSOLVE_` prefix.
Example: `otel.serviceName` → `AUTOSOLVE_OTEL_SERVICENAME`.

See: https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/

</details>

### Migrate database

```bash
go run . migrate up -y

```

### Start scheduler

```bash
go run . schedule
```

Or using Task:

```bash
task up
```

## Architecture

```
cmd/                    CLI entry points (cobra)
internal/
├── bootstrap/          App init (Bootstrap → Wire → Kernel)
├── config/             Typed config structs (viper-backed)
├── domain/             Business logic
│   ├── issue/          Issue parser (Worker)
│   ├── repository/     Repository validator (Preflight)
│   └── spec/           Task specs
│       └── tenants/    Tenant definitions (e.g. RepoTenant)
├── application/        Orchestration layer
│   ├── schedule.go     Two-phase Scheduler
│   ├── planner.go      Per-repo task planning
│   ├── contracts.go    Preflight / Worker interfaces
│   └── registry.go     Task registration
└── infrastructure/     External dependencies
    ├── config/         Config loading (viper reader, validator)
    ├── github/         GitHub API client + rate limiter
    ├── dal/            Data access layer
    │   ├── model/      Domain models
    │   ├── queries/    Raw SQL files (sqlc source)
    │   ├── repositories/ Repository implementations
    │   └── sqlcgen/    sqlc-generated code
    ├── database/       SQLite connection + goose migrator
    ├── logger/         slog setup
    └── telemetry/      OTEL SDK bootstrap
pkg/
└── longrun/            Task orchestration with per-error retry and backoff
```

The scheduler runs in two phases:

1. **Preflights** — one-shot tasks (e.g. validate repository access via GitHub API). All must pass before workers start.
2. **Workers** — long-running interval tasks (e.g. poll and parse issues). If any worker fails permanently, all others are cancelled.

## Commands

| Command                 | Description                               |
|-------------------------|-------------------------------------------|
| `schedule`              | Start the polling daemon                  |
| `migrate up [N]`        | Apply pending migrations (all by default) |
| `migrate up:fresh`      | Drop all tables and re-run all migrations |
| `migrate down <N\|*>`   | Roll back N migrations (or all with `*`)  |
| `migrate status`        | Show migration status                     |
| `migrate create <name>` | Create a new SQL migration file           |
| `migrate redo`          | Roll back and re-apply the last migration |

## Development

```bash
task generate   # sqlc + wire + license headers
task lint        # golangci-lint + license-eye + govulncheck + sqlfluff + sqlcgen type check
task test        # unit tests + benchmarks
```

## Current Status

Epic v1 is in progress — see [Epic: v1 architecture redesign](https://github.com/thumbrise/autosolve/issues/59).

What works today:
- Multi-repo GitHub issue polling with state persistence in SQLite
- Repository validation preflight
- Rate limiting via HTTP transport
- goose migrations + sqlc-generated DAL
- Full OTEL observability (traces, metrics, logs)
- Two-phase scheduler with per-error retry and exponential backoff

## License

[Apache License 2.0](LICENSE)