---
title: Schedule Package
description: How the schedule package orchestrates task registration, partition multiplication, phase planning, and two-phase execution.
---

# Schedule Package

The `internal/application/schedule/` package owns the full lifecycle from task declaration to execution.

## Pipeline

```
Domain specs (TaskSpec, GlobalTaskSpec)
    ↓
Registry DSL — NewTasks()
    ↓  repos.Pack()     → multiply × partitions, wrap closures
    ↓  Preflight()      → mark phase
    ↓  globalTasks()    → wrap without multiplication
    ↓
[]spec.Task (flat list, ready to schedule)
    ↓
Planner — split by Phase
    ↓
Scheduler
    ├── runPreflights() → longrun.NewOneShotTask
    └── runWorkers()    → longrun.NewIntervalTask
```

## Key Types

| Type | File | Role |
|------|------|------|
| `RepositoryTasks` | `repository_tasks.go` | Multiplies specs × configured repos |
| `Preflight()` | `repository_tasks.go` | Marks a spec as preflight phase |
| `globalTasks()` | `global.go` | Wraps global specs without multiplication |
| `join()` | `global.go` | Concatenates task slices |
| `Planner` | `planner.go` | Splits tasks by Phase, validates duplicates |
| `Scheduler` | `schedule.go` | Executes preflights → workers |
| `infraClassifier()` | `schedule.go` | Classifies infra errors for retry |

## Registry DSL

The registry reads as a manifest:

```go
func NewTasks(...) []spec.Task {
    return join(
        repos.Pack(
            Preflight(repoValidator.TaskSpec()),
            issuePoller.TaskSpec(),
            outboxRelay.TaskSpec(),
        ),
        globalTasks(
            issueExplainer.TaskSpec(),
        ),
    )
}
```

Adding a task = one line in the appropriate section. Adding a partition dimension = one new provider + one new section.

## Pack Internals

`Pack(entries ...any)` accepts two types via type switch:

- `repository.TaskSpec` → default `PhaseWork`, lazy repoID resolution
- `repoTask` (from `Preflight()`) → `PhasePreflight`, repoID = 0

Panics on zero Interval (must use `spec.OneShot`) or unknown entry type. This is a conscious trade-off: `any` loses compile-time safety but enables clean DSL syntax. See REVIEW.md "DSL internals may use pragmatic shortcuts."

## Partition Multiplication

For each spec × each configured repository, `Pack` creates a closure that:

1. Lazily resolves `RepositoryID` on first call (work-phase only)
2. Constructs `repository.Partition{Owner, Name, RepositoryID}`
3. Calls the domain's `Work(ctx, partition)` with honest arguments

Domain code never sees closures, config, or lazy resolution. It receives a typed `Partition` and works.

## Phase Model

| Phase | When | Degraded policy | repoID |
|-------|------|-----------------|--------|
| `PhasePreflight` | Before all workers | nil — unknown errors crash | 0 (row may not exist) |
| `PhaseWork` | After preflights complete | Set — survive and retry | Lazy-resolved |

Planner splits `[]spec.Task` by `Phase`. Scheduler runs each phase with its own `longrun.Runner` and Baseline config.

## Extension Points

| What | Where | Planner/Scheduler touched? |
|------|-------|---------------------------|
| New per-repo task | `repos.Pack(...)` +1 line | No |
| New global task | `globalTasks(...)` +1 line | No |
| New partition | New `XxxTasks` provider + registry section | No |
| New phase | +1 `Phase` const, +1 Scheduler runner | Yes |

## Error Classification

`infraClassifier()` checks `apierr` interfaces on errors from infrastructure clients:

- `WaitHinted` with positive duration → Service + explicit wait
- `ServicePressure` → Service
- `Retryable` → Service
- Not classified → nil (baseline handles as Unknown/Degraded)
