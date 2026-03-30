---
title: "autosolve Devlog #5 — Two-Phase Scheduler"
description: The commit that rewired autosolve from single-repo to multi-repo — Planner, Scheduler, Registry, RepoPartition, and the rate limiter RoundTripper.
---

# #5 — Two-Phase Scheduler

> From "one repo, one worker" to "N repos, M tasks, two phases." The commit that changed everything.

## The Trigger

```
d69b909 feat: multi-repo polling with two-phase execution (#72)
```

This is the biggest single commit in the project. One PR that rewired the entire execution model. Before it: one repo, one worker, flat structure. After it: N repos, preflights + workers, Planner + Scheduler + Registry.

Why one commit? Because the old architecture couldn't be incrementally morphed. Single-repo was baked into every layer. It had to be replaced wholesale.

## The Problem

The original daemon polled one repository. Config had `owner` and `repo` fields. The worker had hardcoded references. Adding a second repo meant... duplicating everything?

No. The question was: **what's the unit of work?** The answer became `RepoPartition` — a struct with `Owner`, `Name`, and `RepositoryID`. Every task receives a partition and does its job for that specific repo.

## The Design

Three new concepts appeared in one commit:

### Planner
Takes domain specs and multiplies them by configured repositories. Each repo gets a closure with a captured partition:

```
IssuePoller.TaskSpec() → WorkerSpec{Resource: "issue-poller", Work: ...}

Planner.Workers() → [
  WorkerUnit{Repo: "thumbrise/autosolve", Work: closure(RepoPartition)},
  WorkerUnit{Repo: "thumbrise/otelext",   Work: closure(RepoPartition)},
]
```

Planner is the only place that knows how to map config → partitions → closures. Domain code never knows how many repos exist.

### Scheduler
Two-phase executor. Phase 1: preflights (all must pass). Phase 2: workers (long-running). If any preflight fails, workers never start.

Why two phases? Because you don't want to poll issues for a repo you can't access. `RepositoryValidator` checks GitHub API access and upserts the repo into SQLite. Only after all repos are validated do workers begin.

### Registry
`NewPreflights()` and `NewWorkers()` — explicit registration of all task implementations. Add a new worker? Add one line to `registry.go`, one line to `bindings.go`, run `task generate`.

## What Got Deleted

```
- delete old internal/application/worker/ package
```

The old worker package was a single-repo monolith. Gone. Replaced by domain specs in `internal/domain/spec/workers/` and `internal/domain/spec/preflights/`.

## The Rate Limiter

Multi-repo meant more API calls. The same commit added `RateLimiter` as an `http.RoundTripper` — a transparent interceptor on every outgoing request. Domain code doesn't know it exists. The GitHub client doesn't know it exists. It just works at the transport layer.

## The Bug That Followed

```
9403db6 fix(dal): update repository_id on UpsertIssue conflict
```

One day after the big merge. Pre-existing issues had `repository_id=0` because the upsert's `ON CONFLICT` clause didn't update it. A classic "the migration works for new data but not old data" bug.

## Later: Baseline Replaced TransientRules

The original multi-repo commit had `Transients` on specs — each domain type declared its own retry rules. This worked but was wrong. Domain shouldn't know about retry strategy.

Several commits later (`6186263`, `ac6785f`), Transients were removed from specs entirely. Retry became Baseline's job — configured once on Runner, invisible to tasks. Domain specs became pure: just `Resource`, `Interval`, and `Work`.

```
6186263 refactor(domain): remove Transients from specs,
         remove adaptRateLimit
ac6785f feat(application): baseline on runners,
         InfraClassifier replaces buildRules
```

## What We Learned

The two-phase model is simple but powerful. Preflights guard invariants. Workers do the real work. Planner owns the multiplication. Each layer has one job.

The biggest lesson: **when the architecture doesn't fit, don't patch it — replace it.** The `d69b909` commit was scary (huge diff, everything changed), but the result was clean. Incremental changes would have produced a Frankenstein.

---

*Commits: `d69b909` the big rewrite → `9403db6` first bug → `6186263` + `ac6785f` baseline replaces transients*
