# #3 — Building longrun

> We needed a task runner. We looked at what exists. We built our own. Here's why.

## The Problem

autosolve is a daemon. It runs tasks forever. Tasks fail. Some failures are temporary (network blip), some are permanent (wrong config). The daemon needs to:

- Run N tasks concurrently
- Retry transient failures with backoff
- Kill everything on permanent failure
- Shut down gracefully on SIGTERM

This isn't exotic. Surely there's a library for this?

## What Exists

There are retry libraries (`cenkalti/backoff`, `avast/retry-go`). There are task runners (`uber-go/fx`, `oklog/run`). But nothing that does **per-error retry** — the ability to say "retry this error 5 times with aggressive backoff, but that error 3 times with gentle backoff."

In autosolve, this matters. A GitHub API rate limit (`ErrFetchIssues`) and a local SQLite write error (`ErrStoreIssues`) are both transient, but they need completely different retry strategies. You don't hammer GitHub when they're telling you to slow down. You do hammer your local DB because it'll recover in milliseconds.

## The Evolution

The git history tells the story of three generations:

### Gen 1: Process Runner (`96b9fde`)
Simple goroutine orchestrator. Start processes, wait, shut down. No retry at all — just "run until done or dead." This is where the [shutdown bugs](/devlog/002-graceful-shutdown) lived.

### Gen 2: Task with Strategy (`35e98f0` → `b5ef9b9`)
Added `BackoffConfig`, `Task` type, retry with backoff. But retry was all-or-nothing — one policy for all errors. The "strategy pattern" decomposition (`b5ef9b9`) was an attempt to make it flexible, but it was over-engineered.

### Gen 3: The Redesign (`b1453bf`)
Threw it all away and rebuilt with a clear model:

```go
type TransientRule struct {
    Err        error         // match via errors.Is or errors.As
    MaxRetries int           // per-error budget
    Backoff    BackoffConfig // per-error curve
}
```

One commit: `b1453bf feat(longrun): redesign package — typed constructors, per-error retry, backoff`. Followed immediately by blackbox tests (`d2f750d`) and migration of the issue worker to the new API (`a45ae7c`).

The key insight: **different errors deserve different retry budgets.** Not "is this error retryable?" but "how retryable is this specific error?"

## Then Came Baseline

Much later (`8318999`), after multi-repo landed, we added **Baseline** — a Runner-level safety net that tasks don't even know about:

```
Node    → transport errors (TCP, DNS, timeout) → aggressive retry
Service → remote pressure (rate limit, 5xx)    → gentle retry  
Unknown → not classified                       → degraded mode or crash
```

Baseline runs *after* TransientRules. It's invisible protection. Tasks declare their known errors via rules; Baseline catches everything else.

And **Degraded mode** — when a worker gets an unknown error and Degraded policy is set, it retries internally and never bubbles up to Runner. Like `docker restart: always`. Workers survive. Preflights don't (no Degraded = crash on unknown = fix your config).

## Design Decisions That Stuck

- **Zero external deps** — only `golang.org/x/sync`. The package is designed for extraction into a standalone module.
- **Own backoff** — `math.Pow` based. No reason to import a library for 10 lines of math.
- **Whitelist approach** — empty rules = all errors permanent. You must explicitly opt in to retry.
- **Signals are not the package's job** — Runner takes a `ctx`, caller handles `signal.NotifyContext`.
- **LIFO shutdown** — last added task shuts down first, like `defer`.

## What We Learned

Building your own is expensive. But when the core abstraction doesn't exist in the ecosystem (per-error retry with different budgets), you either shoehorn something or build it right. We built it right, and it became the foundation everything else stands on.

The package is at `pkg/longrun/` — [full README](https://github.com/thumbrise/autosolve/blob/main/pkg/longrun/README.md), [planned features](https://github.com/thumbrise/autosolve/blob/main/pkg/longrun/TODO.md).

---

*Commits: `96b9fde` first runner → `b1453bf` redesign → `8318999` baseline → `4c0e94b` latest fix*
