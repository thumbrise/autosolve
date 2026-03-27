# Multi-repo polling — Implementation spec

## Architecture

```
Domain (IssuePoller, RepositoryValidator)
    ↓ PreflightSpec / WorkerSpec (resource + work(ctx, tenant) + rules)
Planner
    ↓ PreflightUnit / WorkerUnit (repo + замыкание с tenant + cached repoID)
Scheduler
    ↓ name formatting + longrun.Task
Runner (longrun)
```

## Core concepts

- **RepoTenant** — current unit of work. Today it's a repository (owner + name + repoID). Tomorrow it may expand.
- **PreflightSpec** — domain-defined one-shot task spec. Work receives RepoTenant.
- **WorkerSpec** — domain-defined interval task spec. Work receives RepoTenant.
- **Planner** — owns per-repo concept. Iterates config repositories, wraps domain specs into schedule units with closures. Caches repoID in closure on first worker tick.
- **Scheduler** — generic two-phase executor. Phase 1: preflights (all must pass). Phase 2: workers (long-running). Formats task names as `{phase}:{resource}:{owner}/{name}`.
- **Preflight/Worker interfaces** — domain types implement these, return specs. Registered in DI via `NewPreflights()`/`NewWorkers()`.

## Task naming convention

Redis-style colons, general to specific:

```
preflight:repository-validator:thumbrise/autosolve
worker:issue-poller:thumbrise/autosolve
worker:issue-poller:thumbrise/otelext
worker:comment-poller:thumbrise/autosolve   (future)
```

## Config changes

- `Owner`/`Repo` → `Repositories []Repository` (named type with Owner + Name)
- New section `RateLimit` with `MinInterval`
- Validation: repositories non-empty, each element valid, MinInterval > 0

## Rate limiter

- `golang.org/x/time/rate`, burst=1, rate from config
- Wrapped in named type, injected into GitHub Client
- Embedded as `http.RoundTripper` — transparent interceptor on every outgoing HTTP request
- `SetLimit()` for hot-reload readiness

## GitHub Client

- `GetMostUpdatedIssues(ctx, owner, repo, count, since)` — stateless per repository
- Config in struct only for Token and HttpClientTimeout
- Rate limiter via RoundTripper in transport

## Repository Validator (Preflight)

- One-shot: for each repo from config → GitHub API (check existence/access) → upsert to DB via RepositoryRepository
- Permanent error if repo unavailable → Runner kills everything

## Issue Poller (Worker)

- Per-repo via closure: receives RepoTenant with owner, name, repoID
- `GetLastUpdateTime` scoped by `repository_id`
- `mapIssueToModel` sets `RepositoryID` from tenant
- Poller is autonomous — doesn't know about preflight, resolves repoID from DB on first tick (cached in closure)

## RepositoryRepository (DAL)

- SQL queries: `UpsertRepository`, `GetByOwnerName`
- Source: `internal/infrastructure/dal/queries/repositories.sql`

## Error behavior

- Preflight permanent error → app crashes (before workers start)
- Worker transient errors → retry via `longrun.TransientGroup`
- Worker permanent error → `errgroup` kills all → external supervisor restarts

## Files and packages

### Done (skeleton)

- [x] `REVIEW.md` — pure constructors for Wire rule
- [x] `internal/config/github.go` — Repository type, Repositories, RateLimit
- [x] `config.yml.example` — rateLimit section
- [x] `internal/domain/spec/tenants/repo.go` — RepoTenant
- [x] `internal/domain/spec/spec.go` — PreflightSpec, WorkerSpec
- [x] `internal/application/preflight.go` — Preflight interface
- [x] `internal/application/worker.go` — Worker interface
- [x] `internal/application/planner.go` — Planner, PreflightUnit, WorkerUnit
- [x] `internal/application/schedule.go` — two-phase Scheduler

### Done (implementation)

- [x] `internal/infrastructure/github/rate_limiter.go` — wrapper over rate.Limiter + rateLimitedTransport (RoundTripper)
- [x] `internal/infrastructure/github/client.go` — stateless GetMostUpdatedIssues(ctx, owner, repo, ...), rate limiter via transport
- [x] `internal/infrastructure/dal/queries/repositories.sql` — UpsertRepository, GetByOwnerName
- [x] `internal/infrastructure/dal/repositories/repository.go` — RepositoryRepository
- [x] `internal/infrastructure/dal/queries/issues.sql` — GetLastUpdateTime with WHERE repository_id
- [x] `internal/domain/repository/validator.go` — preflight: validate repo via GitHub API + upsert
- [x] `internal/domain/issue/parser.go` — per-repo semantics, accept RepoTenant
- [x] `internal/application/registry.go` — NewPreflights(), NewWorkers() registration
- [x] `internal/bindings.go` — new Wire bindings
- [x] `internal/bootstrap/wire_gen.go` — regenerate
- [x] `go.mod` — golang.org/x/time
- [x] Delete `internal/application/worker/` (old package)
