# application

Orchestrates domain work via a two-phase execution model.

## How it works

```
Scheduler
  ├── Phase 1: Preflights (one-shot, all must pass)
  └── Phase 2: Workers (long-running interval tasks)
```

**Scheduler** runs phases sequentially. If any preflight fails, workers never start.

**Planner** owns the per-repository concept. It takes domain specs and multiplies them
by the configured repositories, producing ready-to-schedule units with closures
that capture tenant context (owner, name, cached repoID).

**Planner** also centralizes retry configuration — domain only declares which errors
are transient (`[]error`), Planner decides how to retry them (max retries, backoff).

## Key types

| Type            | File           | Role                                         |
|-----------------|----------------|----------------------------------------------|
| `Preflight`     | `contracts.go` | Interface for one-shot tasks                 |
| `Worker`        | `contracts.go` | Interface for interval tasks                 |
| `Planner`       | `planner.go`   | Builds per-repo task units from domain specs |
| `Scheduler`     | `schedule.go`  | Executes preflight → worker phases           |
| `NewPreflights` | `registry.go`  | Registers all preflight implementations      |
| `NewWorkers`    | `registry.go`  | Registers all worker implementations         |

## Adding a new task

1. Create a domain type that implements `Preflight` or `Worker` (return a `PreflightSpec` or `WorkerSpec` from `TaskSpec()`).
2. Register it in `registry.go` — add as a parameter to `NewPreflights()` or `NewWorkers()`.
3. Add the constructor to `internal/bindings.go` Wire set.
4. Run `task generate`.

## Task naming

Names are formatted by Scheduler as `{phase}:{resource}:{owner}/{name}`:

```
preflight:repository-validator:thumbrise/autosolve
worker:issue-poller:thumbrise/autosolve
worker:issue-poller:thumbrise/otelext
```

## Extension points: tenants

The current tenant is `RepoTenant` (owner + name + repoID) — all work revolves around
repositories. But this is not a dogma.

`RepoTenant` is defined in `domain/spec/tenants/` and passed to domain work functions
via Planner closures. If a future task needs a different unit of work (e.g. an org-level
tenant, a user-level tenant), the path is:

1. Define a new tenant type in `domain/spec/tenants/`.
2. Define a new spec type with `Work func(ctx, YourTenant) error`.
3. Add a new method to Planner that iterates the appropriate config and builds units.
4. Scheduler calls the new Planner method in the right phase.

Planner is the only place that knows how to map config → tenants → closures.
Domain types never know how many tenants exist or where they come from.

## Future: module system

The current architecture has an implicit pattern: a **tenant** acts as a gravity point
for a cluster of related components — lifecycle phases (preflights, workers), domain
logic, and DAL repositories. Today this cluster is `RepoTenant` with its validator,
issue parser, and repository/issue stores.

This naturally evolves into a **module system** where each tenant type defines a
self-contained module:

```
modules/
  repo/                        ← "repository" module
    tenant.go                  ← RepoTenant
    preflights.go              ← validator, migrations, etc.
    workers.go                 ← issue poller, comment poller, etc.
    dal/                       ← module-specific repositories
    module.go                  ← Module.Register(app)
```

A module would implement a single interface:

```go
type Module interface {
    Preflights() []Preflight
    Workers() []Worker
}
```

The application loads modules explicitly:

```go
modules := []Module{repo.New(), analytics.New(), notifications.New()}
```

This is the same pattern as Linux loadable modules, PHP extensions (`ext-curl`, `ext-pdo`),
or Git subcommands — each module brings its own lifecycle, domain logic, and storage,
and the application provides the execution framework.

**Not implemented yet.** The current flat structure works for the current scale.
When a second tenant type appears, that's the signal to extract the module system.
