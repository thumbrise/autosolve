---
title: autosolve Two-Phase Scheduler
description: How autosolve runs preflights before workers — validate GitHub access first, then start polling. Fail fast, fail safe execution model.
---

# Two-Phase Scheduler

The scheduler runs tasks in two sequential phases. This is the core execution model of autosolve.

## Why Two Phases?

You don't want to start polling issues for a repo you can't access. Preflights validate the world before workers start doing real work.

```
Scheduler.Run(ctx)
  │
  ├─ Phase 1: runPreflights()
  │    All preflights run concurrently via longrun.Runner
  │    ALL must succeed → otherwise app crashes
  │
  └─ Phase 2: runWorkers()
       All workers run concurrently via longrun.Runner
       If any worker dies permanently → all others cancelled
```

## Preflights

One-shot tasks. Currently there's one: `RepositoryValidator`.

For each configured repo, it:
1. Calls GitHub API to verify the repo exists and is accessible
2. Upserts the repo record into SQLite (caches the GitHub repo ID)

If any repo is unreachable → permanent error → app exits. Fix your config and restart.

**No Degraded mode** for preflights. Unknown errors are fatal. This is intentional — preflights guard invariants.

## Workers

Long-running interval tasks. Two kinds:

**Per-repo workers** — multiplied by Planner for each configured repository:
- `IssuePoller` — polls GitHub for updated issues on a timer
- `OutboxRelay` — relays outbox events to the goqite job queue

**Global workers** — run once, not scoped to a repository:
- `IssueExplainer` — consumes the shared goqite queue, calls Ollama, logs AI classification

Workers have **Degraded mode enabled** — unknown errors don't crash the worker, they retry with exponential backoff and loud ERROR logging. Like `docker restart: always`.

## How Registry and Planner Connect Them

The registry DSL declares all tasks. `RepositoryTasks.Pack()` multiplies per-repo specs by configured repositories. Planner splits the resulting `[]spec.Task` by Phase:

```
Registry DSL:
  repos.Pack(
      Preflight(validator.TaskSpec()),   → Phase=Preflight, per repo
      issuePoller.TaskSpec(),            → Phase=Work, per repo
      outboxRelay.TaskSpec(),            → Phase=Work, per repo
  )
  globalTasks(explainer.TaskSpec())      → Phase=Work, single instance

Planner.Preflights() → [
  Task{Name: "preflight:repository-validator:thumbrise/autosolve", ...},
  Task{Name: "preflight:repository-validator:thumbrise/otelext", ...},
]

Planner.Workers() → [
  Task{Name: "worker:issue-poller:thumbrise/autosolve", Interval: 10s, ...},
  Task{Name: "worker:issue-poller:thumbrise/otelext", Interval: 10s, ...},
  Task{Name: "worker:outbox-relay:thumbrise/autosolve", Interval: 5s, ...},
  Task{Name: "worker:outbox-relay:thumbrise/otelext", Interval: 5s, ...},
  Task{Name: "worker:issue-explainer", Interval: 2s, ...},
]
```

See [Schedule Package](./schedule) for DSL details and extension points.

## Retry Policies

Both phases use the same Baseline categories but different Degraded settings:

| Category | Backoff | Used for |
|----------|---------|----------|
| **Node** | 2s → 2min | TCP, DNS, TLS, timeout — network will recover |
| **Service** | 5s → 5min | Rate limits, 5xx — don't kick them while they're down |
| **Degraded** (workers only) | 30s → 5min | Unknown errors — survive, scream, retry |

See [Error Handling & Retry](./error-handling) for the full classification pipeline.

## Source

- [`internal/application/schedule/schedule.go`](https://github.com/thumbrise/autosolve/blob/main/internal/application/schedule/schedule.go) — Scheduler
- [`internal/application/schedule/planner.go`](https://github.com/thumbrise/autosolve/blob/main/internal/application/schedule/planner.go) — Planner
- [`internal/application/schedule/registry.go`](https://github.com/thumbrise/autosolve/blob/main/internal/application/schedule/registry.go) — Registry DSL
- [`internal/application/schedule/repository_tasks.go`](https://github.com/thumbrise/autosolve/blob/main/internal/application/schedule/repository_tasks.go) — Pack / Preflight
