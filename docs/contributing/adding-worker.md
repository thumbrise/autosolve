# Adding a Worker

This is the most common way to extend autosolve. A worker is a long-running interval task that does something for each configured repository.

::: info Example
The existing `IssuePoller` polls GitHub for updated issues every 5 seconds per repo. You might add a `CommentPoller`, `PRPoller`, or `LabelWatcher` — same pattern.
:::

## Steps

### 1. Create the Domain Type

Create a new file in `internal/domain/spec/workers/`:

```go
package workers

type CommentPoller struct {
    githubClient *githubinfra.Client
    // ... your dependencies
    logger *slog.Logger
    cfg    *config.Github
}

func NewCommentPoller(/* dependencies */) *CommentPoller {
    return &CommentPoller{/* assign fields */}
}
```

### 2. Implement the Worker Interface

Return a `WorkerSpec` from `TaskSpec()`:

```go
func (p *CommentPoller) TaskSpec() spec.WorkerSpec {
    return spec.WorkerSpec{
        Resource: "comment-poller",      // used in task name
        Interval: p.cfg.Comments.PollInterval,
        Work:     p.Run,
    }
}

func (p *CommentPoller) Run(ctx context.Context, tenant tenants.RepositoryTenant) error {
    // Your logic here. tenant has Owner, Name, RepositoryID.
    // Return sentinel errors for classifiable failures.
    return nil
}
```

The `Resource` field becomes part of the task name: `worker:comment-poller:thumbrise/autosolve`.

### 3. Register in DI

Add your type to `internal/application/registry.go`:

```go
func NewWorkers(
    issueParser *workers.IssuePoller,
    commentPoller *workers.CommentPoller,  // ← add here
) []Worker {
    return []Worker{
        issueParser,
        commentPoller,  // ← and here
    }
}
```

Add the constructor to `internal/bindings.go`:

```go
var Bindings = wire.NewSet(
    // ...existing bindings...
    workers.NewCommentPoller,  // ← add here
)
```

### 4. Generate

```bash
task generate
```

This runs sqlc + Wire + license headers. Wire will regenerate `wire_gen.go` with your new dependency.

### 5. Done

Your worker will now run for every configured repository. The Planner multiplies your spec by repos automatically. Retry, backoff, degraded mode — all handled by the Baseline on Runner.

## What You Don't Need to Do

- **No retry logic** — Baseline handles transport and service errors
- **No multi-repo logic** — Planner creates per-repo closures
- **No OTEL code** — every invocation is automatically traced
- **No task naming** — Scheduler formats it from your Resource + repo

## Adding a Preflight Instead?

Same pattern, but implement `Preflight` interface and return `PreflightSpec`. Register in `NewPreflights()`. Preflights run once before workers start, and unknown errors are fatal (no Degraded mode).

## Conventions

From [REVIEW.md](https://github.com/thumbrise/autosolve/blob/main/REVIEW.md):
- Pure constructors (no side effects, no goroutines)
- Sentinel errors at domain boundaries
- `context.Context` always first parameter
- Always use `logger.InfoContext(ctx, ...)` — never lose the context
