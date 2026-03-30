---
title: "autosolve Devlog #11 — Killing the God Object"
description: How a Torvalds-style structural review caught a 486-LOC God Object in longrun, and the chain of insights that led to a full pipeline redesign.
---

# #11 — Killing the God Object

> "Splitting files is `mv`, not refactoring." — the moment we almost shipped cosmetic surgery as architecture.

## It Started With Numbers

PR #200 ran a Torvalds-style structural review on `pkg/longrun`. We'd never done one before — normal diff review had always been enough. The numbers said otherwise:

| Metric | Value | Threshold |
|---|---|---|
| File LOC | 486 | > 400 |
| Private methods on Task | 12 | > 10 |
| Cyclomatic complexity | 18 | > 15 |

Three violations. And a lying name: `runWithPolicy` — a function that didn't run with a policy. It was a restart loop. Every reader who trusted that name built wrong assumptions.

None of this surfaced in months of diff review. Tests passed. Code "worked". The rot was structural.

## The Wrong Fix

First instinct: split `task.go` into three files. `task.go` for the struct, `task_loop.go` for the execution loop, `task_baseline.go` for the baseline retry subsystem. Clean, right?

Wrong. We almost shipped it. Then came the question that changed everything:

**"A God Object doesn't stop being a God Object because you spread it across three files."**

Task still had 12 private methods. Still owned two unrelated retry pipelines. Still tracked attempts in two different ways. The files were smaller, but the type was the same junk drawer.

## Three Hidden Entities

We stepped back and asked: what concepts are hiding inside Task, pretending to be fields?

**Hidden entity #1: the backoff algorithm.** `BackoffConfig` was a struct with three fields (`Initial`, `Max`, `Multiplier`), a `Duration()` method, and a `Wait()` method. Three responsibilities in one type: configuration, computation, and IO. But exponential backoff is a mathematical function. Like Fibonacci. Like bubble sort. It doesn't change. It doesn't depend on context. It doesn't need a struct.

Think about it — every utility library (lodash, underscore, Go's `sort` package) provides algorithms as standalone functions you can use anywhere. Our backoff was welded to our type system.

```go
// Before: struct knows about config, math, AND sleeping
BackoffConfig{Initial: 2*time.Second, Max: 2*time.Minute, Multiplier: 2.0}

// After: pure function. Any func(int) time.Duration works.
longrun.Exponential(2*time.Second, 2*time.Minute)
```

Want jitter? Write a function. Want decorrelated backoff? Write a function. Want constant delay for testing? `longrun.Constant(5*time.Second)`. No interface needed. No adapter. Open/Closed at maximum.

**Hidden entity #2: attempt state.** Retry counters lived in two places: `RuleTracker.attempt int` for per-rule tracking, `map[ErrorCategory]int` for per-category baseline tracking. Same concept — "how many times have we retried this thing" — implemented twice, both hardcoded to in-memory.

Process restarts? All counters reset. A worker that spent 2 hours in degraded mode with exponential backoff at attempt 47 suddenly restarts at attempt 0 — minimum backoff — and hammers the recovering service.

We extracted `AttemptStore`: three methods (`Increment`, `Get`, `Reset`), opaque string keys, default `MemoryStore`. One interface replaced two mechanisms. And now users can plug Redis for persistent backoff state.

**Hidden entity #3: the wait operation.** Two implementations of "sleep for N, respect ctx": `BackoffConfig.Wait()` and `Task.waitDuration()`. Same code, different locations. Neither needed to be a method — neither touched any state. Now it's `sleepCtx(ctx, d)` — a package-level function that doesn't know about Task, BackoffFunc, or anything else.

## The Real Fix: Unified Pipeline

With the hidden entities extracted, the real problem became obvious. Task had two retry paths:

```go
// Path 1: TransientRules
if rule := findMatchingRule(err); rule != nil {
    return retryWithRule(ctx, err, rule)
}
// Path 2: Baseline
if baseline != nil {
    return handleBaselineFailure(ctx, err)  // → classify → policyFor → retryWithPolicy
}
return err // permanent
```

Two paths. Two sets of methods. One `if/else` that would grow every time we added a retry strategy.

The fix: `failureHandler` — an internal interface with one method. Two implementations: `ruleFailureHandler` (matches by error, retries with backoff) and `baselineFailureHandler` (classifies, selects policy, retries with metrics). Both return `errSkip` if the error isn't theirs.

Task's `handleFailure` became:

```go
for _, h := range t.handlers {
    if result := h.Handle(ctx, err); result != errSkip {
        return result
    }
}
return err // no handler claimed it → permanent
```

One loop. No branching. Rules first, baseline last. Runner appends baseline handlers at `Add()` time. Task doesn't know the difference.

## The Numbers

| Metric | Before | After |
|---|---|---|
| Task private methods | 12 | 3 |
| Max file LOC | 486 | ~180 |
| Retry pipelines | 2 | 1 |
| Attempt tracking mechanisms | 2 | 1 |
| Backoff: struct with methods | ✓ | pure function |
| Hidden entities | 3 | 0 |

## What We Learned

**Diff review is necessary but not sufficient.** A file can pass every diff review for months while accumulating structural debt. Periodically step back and ask: "Can I draw a line through this type's methods and get two independent groups?" If yes — God Object.

**Splitting files is not refactoring.** If the type still has 12 methods after the split, you moved furniture. You didn't fix the room.

**Algorithms don't belong to systems.** Exponential backoff is math. It shouldn't know about your Task type, your context, your timer. Make it a function. Let it be used anywhere.

**If the same concept is implemented twice, there's a hidden entity.** Two attempt counters = one `AttemptStore`. Two sleep implementations = one `sleepCtx`. The duplication is the signal.

---

*PR #203. Closes #201, #202, #121.*
