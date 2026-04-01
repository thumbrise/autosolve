---
title: "autosolve Devlog #13 — Killing longrun"
description: "How we deleted 20 files, replaced a framework with two primitives, and discovered that resilience is not retry."
---

# #13 — Killing longrun

> "If you can't name it without a buzzword, the abstraction is wrong."

## It Started With a Cleanup

We opened a PR to remove dead code from longrun. Step 5b of a refactoring chain. Routine cleanup. Delete unused types, update imports, move on.

We didn't move on. Every question — "why does this exist?" — led to the next. By the end we had deleted the entire package, rewritten the scheduler, and built a new resilience toolkit from scratch.

This is the story of that transformation.

## longrun Was make_u32_from_two_u16()

Linus Torvalds once rejected a patch that combined two u16 values into a u32. Not because it was wrong — because it made the world worse. An abstraction that adds complexity without adding understanding.

longrun was that abstraction. 20 files. ~1500 lines. Task, Runner, Baseline, Policy, ErrorCategory, ErrorClass, ClassifierFunc, TransientRule, failureHandler, AttemptStore, RuleTracker, Matcher. A glossary of 12 terms for one idea: "if a function fails, maybe try again."

We looked at what the scheduler actually needed from longrun:

```go
runner := longrun.NewRunner(longrun.RunnerOptions{
    Baseline: longrun.NewBaselineDegraded(node, service, degraded, classifier),
})
runner.Add(longrun.NewIntervalTask("poll", 10*time.Second, poller.Run, nil))
runner.Wait(ctx)
```

And what it could be:

```go
grp, ctx := errgroup.WithContext(ctx)
ticker := time.NewTicker(10 * time.Second)
for { fn(ctx); <-ticker.C }
```

Five minutes of errgroup + ticker. Not a framework.

## The Guard Detour

Our first attempt to fix longrun wasn't radical enough. We introduced Guard — an interface that evaluates errors and returns decisions:

```go
type Guard interface {
    Name() string
    Evaluate(err error) Decision
    Reset()
}
```

The pipeline loop called fn, walked guards, first match won. Clean. Explicit. Better than the failureHandler chain.

But Guard was retry-shaped. It could only look at errors *after* the call. Circuit breaker needs to intercept *before*. Timeout needs to wrap the context. Bulkhead needs a semaphore. Guard couldn't express any of these.

We had replaced one retry-only abstraction with another retry-only abstraction. Cleaner, but still wrong.

## The Option Insight

The breakthrough came from a simple question: what does every resilience pattern have in common?

Retry wraps a call with a loop. Timeout wraps a call with a deadline. Circuit breaker wraps a call with a state check. Rate limiter wraps a call with a wait.

They all wrap a call. They all have the same shape:

```go
type Option func(ctx context.Context, call func(context.Context) error) error
```

One type. Full control. Retry implements it with a loop. Timeout implements it with `context.WithTimeout`. Circuit breaker implements it with a state machine check before `call(ctx)`. Any pattern — existing or not yet invented — fits.

This isn't middleware rebranded. Middleware was `func(next) next` — opaque nesting. Option is explicit: you receive `ctx` and `call`, you decide what to do. The pipeline builds a chain, but each Option sees the full picture.

## Two Extension Points, Not One

Options control execution. But OTEL metrics need to *observe* execution without controlling it. A circuit breaker needs *shared state* across calls. These are different concerns.

We split the world:

**Option** — `func(ctx, call) error`. Per-call. Controls execution. Fresh on every `Do()`. No shared state.

**Plugin** — interface with `Name()` and `Events()`. Client-level. Observes execution. Shared state across calls.

```go
// Client level — plugins, shared
client := resilience.NewClient(rsotel.Plugin())

// Call level — options, per-call
client.Call(fn).
    With(retry.On(err, 3, bo)).
    Do(ctx)
```

Two words. Two contracts. Two lifecycles. The compiler enforces the boundary — `NewClient` accepts `Plugin`, `With` accepts `Option`. You can't mix them up.

## The Scheduler Doesn't Know "Retry"

The old scheduler created a `longrun.Runner` with a `Baseline` config. It knew about retry policies, error categories, degraded mode. Too much knowledge for a lifecycle engine.

The new scheduler knows two things: setup runs first, work runs second.

```go
func (s *Scheduler) Run(ctx context.Context) error {
    if err := s.runJobs(ctx, s.plan.Setup, strictRetryOptions()); err != nil {
        return fmt.Errorf("setup: %w", err)
    }
    return s.runJobs(ctx, s.plan.Work, resilientRetryOptions())
}
```

Each job invocation: `s.client.Call(j.Work).With(opts...).Do(ctx)`. Options are created fresh. No shared mutable state. The data race that plagued the old design — shared guards across concurrent goroutines — is impossible by construction.

## WaitHint: Application Decides, Resilience Provides

The old `infraClassifier` extracted `WaitDuration` from errors and passed it through `ErrorClass.WaitDuration`. Three types to carry one `time.Duration`.

Now: `retry.WithWaitHint(serviceWaitHint)`. One function. The application layer knows about `apierr.WaitHinted`. The resilience package provides a slot. Two lines of code replaced an entire classification pipeline.

## Backoff Stayed Perfect

`backoff.Func` — `func(attempt int) time.Duration` — survived every refactoring unchanged. From longrun to Guard to Option. Pure math doesn't care about architecture. Open/closed forever.

## What We Learned

**Ask "why does this exist?" until it hurts.** Every longrun type had a reason when it was created. None of those reasons survived contact with the question.

**Guard was a stepping stone, not a destination.** We needed to build it to see its limits. The detour taught us what the real abstraction looks like.

**Two extension points > one generic interface.** Option and Plugin serve different masters. Collapsing them into one type would have created the same confusion longrun had.

**Per-call state eliminates data races by design.** Not by mutex. Not by documentation. By construction. `Call().With(opts...).Do()` — each call gets fresh options. There is nothing to share.

**The scheduler is a lifecycle engine, not a resilience framework.** It knows phases and order. Resilience is someone else's job.

## The Numbers

- **Deleted**: 20 files, ~1500 lines (entire `pkg/longrun/`)
- **Created**: `pkg/resilience/` — 6 files, ~400 lines
- **Simplified**: scheduler from 125 LOC with Strategy/Phase/map to 70 LOC with explicit phases
- **Glossary**: 12 terms (Task, Runner, Baseline, Policy, ...) → 4 terms (Option, Plugin, Client, CallBuilder)
- **Data races**: fixed by design, not by code

PR #216. The PR that started as "remove dead code" and ended as "remove dead thinking."
