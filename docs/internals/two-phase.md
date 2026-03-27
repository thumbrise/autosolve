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

Long-running interval tasks. Currently: `IssuePoller`.

For each configured repo, it polls GitHub for updated issues on a timer. State is tracked per-repo via sync cursors (ETag, page, since timestamp).

Workers have **Degraded mode enabled** — unknown errors don't crash the worker, they retry with exponential backoff and loud ERROR logging. Like `docker restart: always`.

## How Planner Connects Them

`Planner` is the glue. It takes domain specs and multiplies them by repos:

```
RepositoryValidator.TaskSpec() → PreflightSpec{Resource: "repository-validator", Work: ...}
IssuePoller.TaskSpec()         → WorkerSpec{Resource: "issue-poller", Interval: 5s, Work: ...}

Planner.Preflights() → [
  PreflightUnit{Repo: "thumbrise/autosolve", Work: closure(RepoTenant)},
  PreflightUnit{Repo: "thumbrise/otelext",   Work: closure(RepoTenant)},
]

Planner.Workers() → [
  WorkerUnit{Repo: "thumbrise/autosolve", Interval: 5s, Work: closure(RepoTenant)},
  WorkerUnit{Repo: "thumbrise/otelext",   Interval: 5s, Work: closure(RepoTenant)},
]
```

Scheduler formats task names: `preflight:repository-validator:thumbrise/autosolve`, `worker:issue-poller:thumbrise/otelext`.

## Retry Policies

Both phases use the same Baseline categories but different Degraded settings:

| Category | Backoff | Used for |
|----------|---------|----------|
| **Node** | 2s → 2min | TCP, DNS, TLS, timeout — network will recover |
| **Service** | 5s → 5min | Rate limits, 5xx — don't kick them while they're down |
| **Degraded** (workers only) | 30s → 5min | Unknown errors — survive, scream, retry |

See [Error Handling & Retry](./error-handling) for the full classification pipeline.

## Source

- [`internal/application/schedule.go`](https://github.com/thumbrise/autosolve/blob/main/internal/application/schedule.go) — Scheduler
- [`internal/application/planner.go`](https://github.com/thumbrise/autosolve/blob/main/internal/application/planner.go) — Planner
- [`internal/application/contracts.go`](https://github.com/thumbrise/autosolve/blob/main/internal/application/contracts.go) — Preflight / Worker interfaces
