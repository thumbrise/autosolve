---
title: "autosolve Devlog #12 — The Do Primitive"
description: How extracting doRetry revealed that longrun's real foundation is a single composable resilient call — and why every struct in the current API is scaffolding.
---

# #12 — The Do Primitive

> "Your current DSL isn't bad. It's scaffolding. The building is built. The scaffolding can come down."

## It Started With Duplication

PR #208 extracted `doRetry` — the shared retry algorithm that both `ruleFailureHandler` and `baselineFailureHandler` duplicated. Increment attempt, check budget, compute wait, log, sleep, return nil. The same skeleton, different parameters.

The fix was mechanical: one function, two callers, a `retryParams` struct for the differences. But the extraction revealed something bigger.

`doRetry` doesn't know about Task. Doesn't know about Runner. Doesn't know about Baseline, TransientRules, failureHandler, or any of the machinery we built. It takes a context, an error, some parameters, an AttemptStore, and a logger. That's it. A pure retry decision.

If the core algorithm doesn't need our types — why do our users?

## The Ceremony Problem

Today, the simplest resilient call in longrun requires constructing a Task, inventing a throwaway name, building a slice of TransientRule structs with named fields, and calling Wait. Seven lines of ceremony for one HTTP call that might need three retries.

The name is garbage — nobody will ever read "create-webhook" in a log because it runs once and disappears. The Task creates an AttemptStore, a logger, a handler pipeline. All of it thrown away after one call.

We built a rocket ship for a trip to the grocery store.

## Three Primitives, One Stack

The insight: longrun has three levels of abstraction, but only exposes two.

**Do** — a single resilient call. Block until success, budget exhaustion, or context cancellation. This is what users actually want 90% of the time. It doesn't exist yet.

**Task** — a long-running unit of work. One-shot or interval. This is Do in a loop with lifecycle management.

**Runner** — orchestration of N Tasks. Errgroup, Baseline injection, LIFO shutdown. This is N Tasks coordinated.

Each level builds on the previous. Today we force everyone through Task even when they just need Do.

## Options as Lego

The second insight came from looking at how patterns compose. Every resilience pattern — retry, circuit breaker, bulkhead, hedge, fallback, rate limiter, timeout, load shedding — has the same shape: wrap a function call with behavior. Middleware.

If every pattern is middleware, and Do is a function call, then every pattern is an Option that wraps Do.

Add a line — pattern appears. Remove a line — pattern disappears. Order matters: options read top-to-bottom as an execution pipeline. No struct literals. No named fields. No constructors that validate combinations. Just functions.

This changes everything about the current API.

## The Full Pattern Catalogue

Every resilience pattern is an Option. Here's the complete set:

**retry** — retry on specific errors with independent budgets and backoff curves.

**circuit** — cross-call memory. Shared state machine per endpoint. N failures → circuit opens → instant `ErrOpen` without a network call.

**timeout** — per-call deadline. Independent of context timeout.

**fallback** — if the call fails, run an alternative function.

**bulkhead** — concurrency limiter per resource. Semaphore — the N+1th call blocks or gets `ErrBulkheadFull`. Without bulkhead: dependency throttles → 500 goroutines hang → memory grows → OOM. With bulkhead: 10 hang, the rest wait or fail-fast.

**hedge** — speculative parallel request. If the first call doesn't respond in N, fire a second. First response wins. Google uses this in Bigtable and DNS — p99 tail latency drops dramatically.

**ratelimit** — token bucket. Wait for a token before calling.

**shed** — load shedding. Unlike bulkhead (concurrency cap), shed watches latency or queue depth and starts rejecting *before* the system degrades.

**cache** — resilience cache, not performance cache. If the call fails, return the last successful result. Netflix calls this "fallback to cache". Unlike fallback: no second service needed, you use your own past.

**batch** — group N small calls into one batch call over a time window.

Every pattern has the same shape: wrap a function call with behavior. Middleware.

### Full Combat Example

```go
err := resilience.Do(ctx, func(ctx context.Context) error {
    return paymentGW.Charge(ctx, order)
},
    rsotel.Trace("payment.charge"),                             // span on everything
    shed.OnLatency(2*time.Second, shed.Window(30*time.Second)), // shed under pressure
    bulkhead.Max(20),                                           // max 20 parallel
    ratelimit.Wait(paymentLimiter),                             // wait for token
    circuit.Breaker("payment", circuit.Threshold(10)),          // circuit breaker
    timeout.After(15*time.Second),                              // deadline
    retry.On(ErrGatewayTimeout, 3, backoff.Exponential(2*time.Second, 30*time.Second)),
    retry.On(ErrRateLimit, 5, backoff.Constant(10*time.Second)),
    fallback.To(func(ctx context.Context) error {               // backup gateway
        return backupGW.Charge(ctx, order)
    }),
)
```

Reads top-to-bottom as a pipeline. Each line is one brick. Remove a line — pattern disappears. Add a line — pattern appears. Lego.

## Presets — Batteries Included

Network, DNS, TLS, timeout — these are objectively transient for everyone. Don't make every user write `retry.On(ErrTimeout, ...)`. A preset does it for you.

```go
// Standard resilient HTTP call
err := resilience.Do(ctx, func(ctx context.Context) error {
    return client.Do(ctx, req)
},
    preset.HTTP(),  // retry on timeout, DNS, connection refused, 502/503/429
)

// Resilient gRPC call
err := resilience.Do(ctx, func(ctx context.Context) error {
    return grpcClient.Call(ctx, req)
},
    preset.GRPC(),  // retry on Unavailable, DeadlineExceeded, ResourceExhausted
)

// Resilient database call
err := resilience.Do(ctx, func(ctx context.Context) error {
    return db.Exec(ctx, query)
},
    preset.SQL(),  // retry on connection lost, deadlock, lock timeout
)
```

Inside, a preset is just a set of Options:

```go
func HTTP() resilience.Option {
    return resilience.Compose(
        retry.On(ErrTimeout, 3, backoff.Exponential(500*time.Millisecond, 10*time.Second)),
        retry.On(ErrConnectionRefused, 3, backoff.Exponential(1*time.Second, 15*time.Second)),
        retry.On(ErrDNS, 2, backoff.Constant(2*time.Second)),
        retry.OnHTTP(502, 503, 3, backoff.Exponential(1*time.Second, 30*time.Second)),
        retry.OnHTTP(429, 5, backoff.FromHeader("Retry-After")),
    )
}
```

Presets compose with custom Options. Preset is the default. Custom is the override. `Compose` stacks them. Order matters: a custom `retry.On` for a specific error overrides the preset's.

```go
err := resilience.Do(ctx, func(ctx context.Context) error {
    return github.FetchIssues(ctx, repo)
},
    preset.HTTP(),                                                 // standard transients
    retry.On(ErrRateLimit, 10, backoff.Constant(60*time.Second)),  // custom on top
    circuit.Breaker("github", circuit.Threshold(5)),
    rsotel.Trace("github.fetch"),
)
```

## What Dies

**TransientRule struct.** Three named fields become three positional arguments to `retry.On`. The struct existed because we needed to name things while we figured out what the parameters were. We figured it out. The names can go.

**Policy struct.** Two fields become two arguments to `baseline.OnNode`. Same story.

**Baseline struct.** A config bag with Policies map, Default pointer, and Classify function becomes individual Options on Runner. No more `NewBaseline` vs `NewBaselineDegraded` — the presence or absence of `baseline.OnUnknown` determines degraded mode.

**failureHandler interface.** The internal handler pipeline becomes Option composition. `ruleFailureHandler` becomes `retry.On`. `baselineFailureHandler` becomes a set of `baseline.On*` Options that Runner injects at `Add()` time. The interface served its purpose — it unified two retry paths into one loop. Now the loop itself is `Do`.

**handleFailure method.** The for-loop over handlers becomes the middleware chain inside Do. Same semantics, no custom interface needed.

## What Survives

**doRetry.** The core algorithm. It becomes the engine inside `retry.On`.

**AttemptStore.** The persistent retry state interface. Still needed — Options that do retry need to track attempts.

**BackoffFunc.** Pure function, already perfect. Moves to `backoff/` sub-package unchanged.

**sleepCtx.** Context-aware sleep. Utility, stays.

**The pipeline priority model.** Task-level Options first, Runner-level Options last. Same as today: rules first, baseline last. The principle survives even though the mechanism changes.

## Circuit Breaker Fills the Gap

One thing longrun never had: memory across tasks. Degraded mode remembers failures within one task's retry series. But if three tasks all hit the same GitHub API and all three get 503 — each sees one failure. Nobody sees three.

Circuit breaker adds cross-task memory. Shared state machine per endpoint. Ten failures across any combination of tasks → circuit opens → every task gets instant ErrOpen without a network call. This is the missing piece between "retry harder" and "stop trying."

It stacks with everything we have. Circuit breaker sits outside retry. If the circuit is open, retry never starts. If closed, retry works normally and failures feed back into the circuit. WaitHint still works inside retry — it's orthogonal. Degraded mode still works — it's a classification strategy, not a retry mechanism.

## How Do Replaces the Internals

Today, Task has a hand-rolled failure pipeline:

```go
func (t *Task) restartLoop(ctx context.Context) error {
    for {
        err := t.runLoop(ctx)
        if err == nil { return nil }

        retryErr := t.handleFailure(ctx, err, hadProgress)
        if retryErr != nil { return retryErr }  // permanent
        // retry — loop continues
    }
}
```

`handleFailure` is a hand-built pipeline of `failureHandler` interfaces. Each handler is a separate struct with fields, state, logic.

With Do:

```go
func (t *Task) restartLoop(ctx context.Context) error {
    for {
        err := resilience.Do(ctx, t.work, t.opts...)
        if err != nil { return err }
        if t.interval == 0 { return nil }
        sleepCtx(ctx, t.interval)
    }
}
```

`handleFailure` disappears. `failureHandler` interface disappears. `ruleFailureHandler`, `baselineFailureHandler` — disappear. All retry logic lives in Options assembled at Task construction time.

### TransientRules → Options

Current:

```go
longrun.NewIntervalTask("poll", 10*time.Second, poller.Run, []longrun.TransientRule{
    {Err: ErrFetchIssues, MaxRetries: 5, Backoff: longrun.Exponential(2*time.Second, 60*time.Second)},
    {Err: ErrStoreIssues, MaxRetries: longrun.UnlimitedRetries, Backoff: longrun.Exponential(100*time.Millisecond, 2*time.Second)},
})
```

Future:

```go
longrun.NewIntervalTask("poll", 10*time.Second, poller.Run,
    retry.On(ErrFetchIssues, 5, backoff.Exponential(2*time.Second, 60*time.Second)),
    retry.On(ErrStoreIssues, retry.Unlimited, backoff.Exponential(100*time.Millisecond, 2*time.Second)),
)
```

`TransientRule` struct disappears. Each `retry.On` is an Option. Same semantics, less ceremony. No struct literal with named fields — positional arguments in a clear order: error, budget, backoff.

### Baseline → Options

Current:

```go
longrun.NewRunner(longrun.RunnerOptions{
    Baseline: longrun.NewBaselineDegraded(
        longrun.Policy{Retries: 10, Backoff: longrun.Exponential(2*time.Second, 2*time.Minute)},
        longrun.Policy{Retries: 5, Backoff: longrun.Exponential(5*time.Second, 5*time.Minute)},
        longrun.Policy{Backoff: longrun.Exponential(30*time.Second, 5*time.Minute)},
        myClassifier,
    ),
})
```

Future:

```go
longrun.NewRunner(
    baseline.Classify(myClassifier),
    baseline.OnNode(10, backoff.Exponential(2*time.Second, 2*time.Minute)),
    baseline.OnService(5, backoff.Exponential(5*time.Second, 5*time.Minute)),
    baseline.OnUnknown(baseline.Unlimited, backoff.Exponential(30*time.Second, 5*time.Minute)),
)
```

`Baseline` struct, `Policy` struct, `NewBaseline`, `NewBaselineDegraded` — all disappear. Runner accepts Options. Each `baseline.On*` is an Option that Runner injects into every Task at `Add()` time.

### Option Merging at Add()

```go
runner := longrun.NewRunner(
    baseline.OnNode(10, backoff.Exponential(2*time.Second, 2*time.Minute)),
    circuit.Breaker("github-api", circuit.Threshold(10)),
)

// Task adds its own Options on top of Runner-level
runner.Add(longrun.NewIntervalTask("poll", 10*time.Second, poller.Run,
    retry.On(ErrFetchIssues, 5, backoff.Exponential(2*time.Second, 60*time.Second)),
    timeout.After(30*time.Second),
    rsotel.Trace("poll.issues"),
))
```

At `Add()` Runner merges: task Options + runner Options. Task-level goes first (priority), runner-level is the fallback. Same principle as today: rules first, baseline last.

## The Full DSL

```go
runner := longrun.NewRunner(
    // Runner-level — applied to all tasks
    baseline.Classify(infraClassifier),
    baseline.OnNode(10, backoff.Exponential(2*time.Second, 2*time.Minute)),
    baseline.OnService(5, backoff.Exponential(5*time.Second, 5*time.Minute)),
    baseline.OnUnknown(baseline.Unlimited, backoff.Exponential(30*time.Second, 5*time.Minute)),
    circuit.Breaker("github-api", circuit.Threshold(10)),
)

runner.Add(longrun.NewOneShotTask("migrate", db.AutoMigrate))

runner.Add(longrun.NewIntervalTask("poll-issues", 10*time.Second, poller.Run,
    retry.On(ErrFetchIssues, 5, backoff.Exponential(2*time.Second, 60*time.Second)),
    retry.On(ErrStoreIssues, retry.Unlimited, backoff.Exponential(100*time.Millisecond, 2*time.Second)),
    timeout.After(30*time.Second),
    rsotel.Trace("poll.issues"),
))

runner.Add(longrun.NewIntervalTask("poll-comments", 30*time.Second, commentPoller.Run,
    retry.On(ErrFetchComments, 3, backoff.Exponential(1*time.Second, 30*time.Second)),
    bulkhead.Max(5),
    rsotel.Trace("poll.comments"),
))

err := runner.Wait(ctx)
```

Everything reads. No struct literals. No named fields. No `TransientRule`, `Policy`, `Baseline`, `NewBaselineDegraded`. Functions with clear names and positional arguments.

- **migrate** — bare task, no Options. Gets only runner-level baseline and circuit breaker.
- **poll-issues** — own retry rules + timeout + tracing on top of runner-level.
- **poll-comments** — own retry + bulkhead (max 5 parallel calls to comment API) + tracing.

Each line is one brick. Lego.

## Repository Layout

```
thumbrise/resilience/
├── go.mod              // core: Do, Option, Compose
├── backoff/            // BackoffFunc, Exponential, Constant
├── retry/              // retry.On, AttemptStore
├── circuit/            // circuit.Breaker
│   └── go.mod
├── bulkhead/           // bulkhead.Max
├── hedge/              // hedge.After
├── shed/               // shed.OnLatency
├── fallback/           // fallback.To
├── timeout/            // timeout.After
├── ratelimit/          // ratelimit.Wait
│   └── go.mod          // depends on rate limiter impl
├── preset/             // preset.HTTP, preset.SQL
├── otel/               // rsotel.Trace
│   └── go.mod          // depends on OTel SDK
└── grpc/               // preset.GRPC
    └── go.mod          // depends on google.golang.org/grpc/codes
```

Separate `go.mod` only where there's an external dependency (`circuit` for state machine, `ratelimit` for token bucket, `otel` for OTel SDK, `grpc` for gRPC codes). Everything else — zero dependencies, same module.

## Why Not Now

We're unreleased. We have the right to break everything. But Do is a foundation change — it restructures how Task and Runner work internally. The current API is proven and tested. The new one is a vision.

The path: finish hardening the current API inside autosolve (Phase 1). Extract as `thumbrise/resilience` with Do as the entry point (Phase 2). Add patterns incrementally (Phase 3). Ship presets (Phase 4).

`doRetry` was the first step. It proved the algorithm is independent of the types. The rest follows.

PR #208. Spawned issue for `resilience.Do` primitive.