---
title: Adding a Task to autosolve
description: Step-by-step guide to adding a new task in autosolve — create a domain spec, register in the DSL, run codegen, done.
---

# Adding a Task

This is the most common way to extend autosolve. Three steps: declare, register, generate.

::: info Example
The existing `IssuePoller` polls GitHub for updated issues every 10 seconds per repo. You might add a `CommentPoller`, `PRPoller`, or `LabelWatcher` — same pattern.
:::

## Per-Repository Task

### 1. Declare

Create a new file in `internal/domain/spec/repository/`:

```go
package repository

type CommentPoller struct {
    cfg          *config.Github
    githubClient *githubinfra.Client
    logger       *slog.Logger
}

func NewCommentPoller(cfg *config.Github, githubClient *githubinfra.Client, logger *slog.Logger) *CommentPoller {
    return &CommentPoller{cfg: cfg, githubClient: githubClient, logger: logger}
}

func (p *CommentPoller) TaskSpec() TaskSpec {
    return TaskSpec{
        Resource: "comment-poller",
        Interval: p.cfg.Comments.PollInterval,
        Work:     p.Run,
    }
}

func (p *CommentPoller) Run(ctx context.Context, partition Partition) error {
    // partition has Owner, Name, RepositoryID — honest, typed, no context magic.
    // Return sentinel errors for classifiable failures.
    return nil
}
```

Domain is naive — it declares work and its partition need. No retry logic, no multi-repo logic, no lifecycle phase.

### 2. Register

Add one line to `internal/application/schedule/registry.go`:

```go
func NewTasks(
    repos *RepositoryTasks,
    // ...existing params...
    commentPoller *repository.CommentPoller,  // ← add param
    // ...
) []spec.Task {
    return join(
        repos.Pack(
            // ...existing specs...
            commentPoller.TaskSpec(),  // ← add here
        ),
        globalTasks(
            // ...
        ),
    )
}
```

Add the constructor to `Bindings` in the same file:

```go
var Bindings = wire.NewSet(
    // ...existing bindings...
    repository.NewCommentPoller,  // ← add here
)
```

### 3. Generate

```bash
task generate
```

Done. Your task runs for every configured repository. `RepositoryTasks.Pack()` multiplies it automatically.

The `Resource` field becomes part of the task name: `worker:comment-poller:thumbrise/autosolve`.

## Global Task

Global tasks are not multiplied per repository. They run once, consuming shared resources (e.g. a job queue).

Same three steps, but:
- Return `spec.GlobalTaskSpec` instead of `spec.TaskSpec`
- `Work` takes only `context.Context` — no partition parameter
- Register in `globalTasks(...)` instead of `repos.Pack(...)`

```go
func (e *QueueDrainer) TaskSpec() spec.GlobalTaskSpec {
    return spec.GlobalTaskSpec{
        Resource: "queue-drainer",
        Interval: 5 * time.Second,
        Work:     e.Run,
    }
}
```

Example: `IssueExplainer` — consumes the shared goqite queue, calls Ollama.

## Preflight Task

A preflight is a one-shot task that runs before all workers. Use it for environment setup (validate repos, check API access).

Same as per-repository task, but wrap with `Preflight()` in the registry:

```go
repos.Pack(
    Preflight(repoValidator.TaskSpec()),  // ← runs once, before workers
    commentPoller.TaskSpec(),
)
```

Domain doesn't know it's a preflight — the registry decides the lifecycle phase.

Preflights receive `Partition` with `RepositoryID=0` (the DB row may not exist yet). Unknown errors are fatal (no Degraded mode).

## What You Don't Need to Do

- **No retry logic** — Baseline handles transport and service errors
- **No multi-repo logic** — `Pack()` multiplies per repo automatically
- **No OTEL code** — every invocation is automatically traced
- **No task naming** — generated from Resource + partition label
- **No lifecycle phase** — registry decides preflight vs worker

## Adding a New Partition Dimension?

See the `RepositoryTasks` pattern. Create a new provider (e.g. `OrganizationTasks`), add a new section in the registry:

```go
return join(
    repos.Pack(...),
    orgs.Pack(            // ← new partition dimension
        auditScanner.TaskSpec(),
    ),
    globalTasks(...),
)
```

## Conventions

From [REVIEW.md](https://github.com/thumbrise/autosolve/blob/main/REVIEW.md):
- Pure constructors (no side effects, no goroutines)
- Sentinel errors at domain boundaries
- `context.Context` always first parameter
- Always use `logger.InfoContext(ctx, ...)` — never lose the context
