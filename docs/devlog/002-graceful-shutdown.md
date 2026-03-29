---
title: "autosolve Devlog #2 — Graceful Shutdown"
description: Three shutdown bugs in Go — born-cancelled contexts, deadlocked hooks, and broken grace periods. How autosolve fixed them with context.WithoutCancel.
---

# #2 — Graceful Shutdown Is Hard

> Three bugs in a row. All about context. All about shutdown. A rite of passage for any Go daemon.

## The Setup

The first version of `longrun.Runner` was simple: start goroutines, wait for them, shut down on signal. What could go wrong?

Everything.

## Bug 1: The Born-Cancelled Context

```
f7dc8be fix: derive shutdown context from caller context
         to preserve values and avoid born-cancelled timeout
```

The shutdown path created a fresh `context.Background()` with a timeout. Sounds reasonable — you want a clean context for cleanup, right?

Wrong. The parent context was already cancelled (that's why we're shutting down). The new context was fine. But we lost all the values from the parent — trace IDs, logger context, everything that makes observability work. And worse: in some paths the timeout context was derived from the already-cancelled parent, so it was born dead. A 30-second grace period that expired instantly.

**Fix:** derive from the caller's context but strip the cancellation with `context.WithoutCancel`.

## Bug 2: The Deadlock

```
96143ea fix: run shutdownProcesses concurrently on context cancel
         to prevent deadlock
a68010e fix: prevent shutdown deadlock and use descriptive
         context names in Runner.Wait
```

Two commits, same root cause. The shutdown sequence was:

1. Context cancelled → errgroup returns
2. Run shutdown hooks sequentially
3. But one hook was waiting for a goroutine that was waiting for the errgroup

Classic deadlock. The shutdown hook couldn't complete because the thing it was shutting down was blocked on the same synchronization primitive.

**Fix:** run shutdown hooks after `grp.Wait()` returns, never concurrently with running tasks. This became a design rule: *"Shutdown after all tasks stop — shutdown hooks run after `grp.Wait()`, never concurrently with running tasks."*

## Bug 3: The Grace Period

```
0a02b3d fix: use WithoutCancel for shutdown context
         to guarantee 30s grace period
```

Even after fixing bugs 1 and 2, the grace period still didn't work reliably. The shutdown context was derived from a cancelled parent, so `context.WithTimeout(cancelledCtx, 30s)` gave us... a cancelled context with a 30-second timeout that was already exceeded.

`context.WithoutCancel` was the answer. It preserves values but ignores the parent's cancellation. The shutdown gets its own 30-second window, guaranteed.

## The Pattern That Emerged

These three bugs crystallized into rules that ended up in `REVIEW.md`:

- **Never create contexts from scratch** — no `context.Background()` mid-call-chain
- **Respect parent cancellation** — derive via `WithTimeout`, `WithCancel`
- **Graceful shutdown exception** — use `context.WithoutCancel(parent)` + own timeout

And in `longrun.Runner`:
```go
ctxShutdown, cancel := context.WithTimeout(
    context.WithoutCancel(ctx),  // values: yes, cancellation: no
    r.opts.ShutdownTimeout,
)
```

## What We Learned

Graceful shutdown in Go looks trivial until you actually do it. The `context` package is powerful but unforgiving — one wrong derivation and your entire shutdown path is broken. The worst part: these bugs are silent. No panics, no errors. The process just hangs or exits too early, and your logs say nothing because the logger context is already dead.

Three bugs, three fixes, one lesson: **treat your shutdown path with the same rigor as your hot path.**

---

*Commits: `f7dc8be` → `0a02b3d` (3 fixes over ~2 weeks)*
