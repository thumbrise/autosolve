# longrun
A self-contained Go package for long-running tasks with interval scheduling, per-error retry, and exponential backoff.

Zero external dependencies beyond `golang.org/x/sync`.

## Overview
`longrun` provides two primitives:
- **Task** — a self-contained unit of work: one-shot or interval, with optional per-error retry and backoff. Can be used standalone via `Wait(ctx)`.
- **Runner** — orchestrates N tasks. When any task dies permanently, cancels all others and runs shutdown hooks.

## Task execution model
```
Task.Wait(ctx)
  └→ runWithPolicy (restart loop + backoff)
       └→ runLoop (ticker or one-shot)
            └→ runOnce (single invocation ± timeout)
```

## Constructors
### NewOneShotTask
Execute once. If `rules` is nil — no retries, any error is fatal.
```go
// Simple one-shot (useful in Runner for coordination)
task := longrun.NewOneShotTask("migrate", db.AutoMigrate, nil)

// One-shot with retry
task := longrun.NewOneShotTask("migrate", db.AutoMigrate, []longrun.TransientRule{
    {Err: ErrConnRefused, MaxRetries: 5, Backoff: longrun.DefaultBackoff()},
})
```

### NewIntervalTask
Ticker loop. If `rules` is nil — any error kills the task.
```go
// Interval without retry
task := longrun.NewIntervalTask("healthcheck", 30*time.Second, check, nil)

// Interval with per-error retry
task := longrun.NewIntervalTask("poll", 10*time.Second, w.poll, []longrun.TransientRule{
    // GitHub API — might be under load, retry carefully
    {Err: ErrFetchIssues, MaxRetries: 5, Backoff: longrun.BackoffConfig{
        Initial: 2 * time.Second, Max: 60 * time.Second, Multiplier: 3.0,
    }},
    // Local DB — not loaded, retry aggressively
    {Err: ErrStoreIssues, MaxRetries: longrun.UnlimitedRetries, Backoff: longrun.BackoffConfig{
        Initial: 100 * time.Millisecond, Max: 2 * time.Second, Multiplier: 2.0,
    }},
}, longrun.WithLogger(logger))
```

## TransientRule
Each rule binds an error to its retry settings. Different errors can have different retry budgets and backoff curves.
```go
type TransientRule struct {
    Err        any           // error sentinel (errors.Is) or pointer-to-type (errors.As)
    MaxRetries int           // 0 = default (3), -1 = unlimited
    Backoff    BackoffConfig
}
```
The `Err` field accepts two forms:
- `error` value (sentinel) → matched via `errors.Is`
- `*T` where T implements error → matched via `errors.As`

Examples:
```go
{Err: ErrTimeout}           // sentinel → errors.Is
{Err: (*net.OpError)(nil)}  // pointer-to-type → errors.As
```
Passing nil or an unsupported type panics at construction time:
`"longrun.NewMatcher: errVal must be an error value or pointer to error type (*T), got: %T"`

Each rule has its own attempt budget. `MaxRetries` limits **consecutive** failures for a given rule. When an interval task completes a successful tick, **all** rule trackers reset to zero — so intermittent failures separated by successful ticks never accumulate toward `MaxRetries`. For one-shot tasks the budget is never reset mid-execution.

## Building blocks
The package exposes low-level building blocks used internally by Task. They are exported for testability and advanced use cases, but most users should only create Tasks.

| Type | Purpose |
|---|---|
| `Matcher` | Compiles an `any` error pattern into `errors.Is`/`errors.As` check |
| `RuleTracker` | Per-rule retry budget with `OnFailure()`/`Reset()` |
| `BackoffConfig` | Exponential backoff with `Duration(attempt)` and `Wait(ctx, attempt)` |

## Options
```go
longrun.WithTimeout(30 * time.Second)  // per-invocation timeout
longrun.WithShutdown(server.Shutdown)  // graceful shutdown hook
longrun.WithDelay(5 * time.Second)     // delay before first execution
longrun.WithLogger(logger)             // custom slog.Logger
```

### WithDelay
Delays the first execution by the given duration.
- For interval tasks: first tick fires after delay, then every interval.
- For one-shot tasks: execution starts after delay.
- Delay is independent of interval.

## Runner
Orchestrates multiple tasks. Does NOT handle OS signals — pass a cancellable context.
```go
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()

runner := longrun.NewRunner(longrun.RunnerOptions{Logger: logger})
runner.Add(migrate)
runner.Add(poll)
runner.Add(server)

err := runner.Wait(ctx)
```
When any task returns a permanent error, Runner cancels all remaining tasks via context, waits for all goroutines to finish, then runs shutdown hooks in LIFO order (reverse of Add).

## Baseline

Baseline is a set of policies that Runner silently applies to every task. Tasks don't know about baseline — it's configured once on Runner.

```go
runner := longrun.NewRunner(longrun.RunnerOptions{
    Logger: logger,
    Baseline: longrun.Baseline{
        Node:    longrun.Policy{Backoff: longrun.Backoff(2*time.Second, 2*time.Minute)},
        Service: longrun.Policy{Backoff: longrun.Backoff(5*time.Second, 5*time.Minute)},
        Degraded: &longrun.Policy{Backoff: longrun.Backoff(30*time.Second, 5*time.Minute)},
        Classify: infraClassifier,
    },
})
```

### Error categories

| Category | Meaning | Policy |
|----------|---------|--------|
| **Node** | Transport-level failure (TCP, DNS, TLS, timeout) | Aggressive retry — network will recover |
| **Service** | Remote service under pressure (rate limit, 5xx) | Gentle retry — don't kick them while they're down |
| **Unknown** | Not recognized by any classifier | Degraded policy (if set) or permanent error |

### Classification pipeline

```
err from work()
  │
  ├─ [1] Built-in transport classify (net.OpError, DNS, timeout, EOF → Node)
  ├─ [2] User classifier via Baseline.Classify (apierr interfaces → Service)
  └─ [3] Not classified → Unknown
         Degraded != nil → retry with loud ERROR log
         Degraded == nil → permanent error
```

Built-in transport classifier depends only on stdlib. User classifier is application-level (e.g. checks `Retryable`, `WaitHinted`, `ServicePressure` interfaces on errors).

### Degraded mode

Task-level behavior, not Runner-level. When a task gets an unknown error and Degraded policy is set:
- Retries internally with Degraded backoff
- Never returns the error to Runner — errgroup contract preserved
- Logs at ERROR level on every retry
- Like Docker `restart: always`

When Degraded is nil (e.g. preflights), unknown errors are permanent — crash early, fix your config.

### Wait duration override

When `ClassifierFunc` returns `ErrorClass.WaitDuration > 0`, the task sleeps exactly that duration instead of using exponential backoff. Typical source: `Retry-After` header on HTTP 429.

## Package structure
```
pkg/longrun/
├── backoff.go       BackoffConfig, DefaultBackoff(), Backoff() constructor
├── baseline.go      Baseline, Policy, ErrorCategory, ClassifierFunc, ErrorClass
├── classify.go      Built-in transport classifier (net, DNS, timeout, EOF)
├── matcher.go       Matcher — errors.Is / errors.As pattern matching
├── metrics.go       OTel metrics (degraded_total, degraded_duration, baseline_retry_total)
├── tracker.go       RuleTracker — per-rule retry budget
├── rule.go          TransientRule (user config) + ruleState (internal)
├── option.go        Functional options (WithTimeout, WithDelay, ...)
├── task.go          Task, NewOneShotTask, NewIntervalTask, handleFailure pipeline
├── runner.go        Runner, NewRunner, Add (passes Baseline), Wait, LIFO shutdown
├── *_test.go        Blackbox tests (package longrun_test)
├── README.md
└── TODO.md
```

## Observability

Every invocation of the work function is automatically wrapped in an OpenTelemetry span inside `runOnce`. The span is named after the task (`name` parameter from the constructor).
- **No SDK configured** → `otel.Tracer` returns a no-op tracer, zero overhead.
- **SDK configured** → every invocation, retry, and error is visible in the tracing backend.
  The span records errors automatically: `span.RecordError(err)` + `span.SetStatus(codes.Error, ...)` on failure. Users get full observability without writing any OTEL code in their work functions.

```text
[longrun/task: "polling issues"]           ← automatic span from longrun
└─[IssuePolling.work]                    ← user's child span (optional)
└─[Parser.Run]                       ← domain span (optional)
└─[SQL INSERT]                   ← infra span (optional)
```

Combined with a `slog.Handler` that extracts span context (trace_id, span_id, scope), every log line emitted via `logger.InfoContext(ctx, ...)` is automatically correlated with the active trace — zero boilerplate in business code.

### Metrics

Baseline retries emit OTel metrics (no-op when SDK is not configured):

| Metric | Type | Labels | Purpose |
|--------|------|--------|---------|
| `longrun_baseline_retry_total` | Counter | `task`, `category` | Each retry via baseline (node/service/degraded) |
| `longrun_degraded_total` | Counter | `task` | Each retry in degraded mode |
| `longrun_degraded_duration_seconds` | Histogram | `task` | Time spent in a single degraded wait |

Enables alerting: "task X in degraded for 10 minutes".

## Design decisions
- **Baseline + TransientRules** — two layers of retry. Baseline is invisible protection configured on Runner (transport + classifier + degraded). TransientRules are explicit per-task overrides for specific errors. Baseline runs after rules — if no rule matches, baseline classifies.
- **Transient errors whitelist** — empty rules = all errors permanent (unless baseline is configured). Lower layers provide sentinel errors, orchestrator decides what to retry.
- **Per-error retry** — different errors can have different MaxRetries and BackoffConfig. Careful retry for loaded external APIs, aggressive retry for local resources.
- **Own backoff** — `math.Pow` based, no external dependencies.
- **Signals are not the package's responsibility** — Runner accepts ctx, caller handles signals.
- **Shutdown after all tasks stop** — shutdown hooks run after `grp.Wait()`, never concurrently with running tasks.
- **LIFO shutdown** — last added task shuts down first (like `defer`). Will transition to reverse topological order when DependsOn is implemented.
- **Typed nil pointer for type matching** — `(*MyError)(nil)` triggers `errors.As` path; non-nil error values trigger `errors.Is`. Checked before `error` interface to avoid ambiguity.

## MaxRetries semantics
| Value | Meaning |
|-------|---------|
| `0` (zero-value) | `DefaultMaxRetries` (3) — safe default |
| `-1` (`UnlimitedRetries`) | No limit — explicit opt-in |
| `> 0` | Exact retry count |

## Future
See [TODO.md](TODO.md) for planned features.