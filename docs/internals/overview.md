---
title: autosolve Architecture Overview
description: High-level architecture of autosolve — bootstrap, Wire DI, two-phase scheduler, Planner, longrun Runner, and the directory structure explained.
---

# Architecture Overview

## The Big Picture

```
config.yml
    ↓
Bootstrap (load config → Wire DI → Kernel)
    ↓
Registry DSL (declare tasks, multiply by partitions)
    ↓
Planner (split by Phase)
    ↓
Scheduler
  ├── Phase 1: Preflights (one-shot, all must pass)
  └── Phase 2: Workers (interval tasks)
              ↓
          longrun.Runner (per-error retry, backoff, degraded mode)
```

## Directory Structure

```
cmd/                    CLI entry points (cobra)
internal/
├── bootstrap/          App init (Bootstrap → Wire → Kernel)
├── config/             Typed config structs (viper-backed)
├── domain/             Business logic
│   ├── repositories.go RepositoryStore interface (domain contract)
│   ├── entities/       Issue, Cursor, User
│   └── spec/           Task specs
│       ├── repository/ Partition, TaskSpec, Validator, IssuePoller, OutboxRelay
│       └── global/     IssueExplainer
├── application/        Orchestration layer
│   └── schedule/
│       ├── schedule.go     Two-phase Scheduler
│       ├── planner.go      Phase-based task splitting
│       ├── registry.go     Declarative task registry (DSL)
│       ├── repository_tasks.go  Per-repo task multiplication
│       └── global.go       Global task helpers
└── infrastructure/     External dependencies
    ├── github/         GitHub API client + rate limiter
    ├── dal/            Data access (sqlc-generated)
    ├── database/       SQLite + goose migrations
    └── telemetry/      OTEL SDK bootstrap
pkg/
└── longrun/            Task orchestration (standalone package)
```

## Key Design Decisions

### Domain Is Naive

Domain types (`RepositoryValidator`, `IssuePoller`) declare *what* to do via `TaskSpec`. They receive their partition as an honest function argument — no context injection, no lifecycle awareness. They don't know about retry, backoff, multi-repo multiplication, or error classification. That's all handled by the application layer.

### Registry DSL Owns the Manifest

The registry (`NewTasks`) reads like a table of contents: what runs, under which partition, in which phase. `repos.Pack()` multiplies per-repo tasks. `globalTasks()` wraps partition-free tasks. `Preflight()` marks one-shot setup tasks. Adding a task = one line.

### Partition Providers Own Multiplication

`RepositoryTasks.Pack()` takes domain specs and multiplies them by configured repositories. Each repo gets its own closure with a captured partition. Domain code receives a `repository.Partition` and does its job — it never knows how many repos exist.

### Error Classification Pipeline

```
err from work()
  ├─ Built-in transport classify (net errors → Node)
  ├─ User classifier (apierr interfaces → Service)
  └─ Not classified → Unknown
       Degraded policy set → retry with loud logging
       Degraded policy nil → permanent error (crash)
```

Preflights have no Degraded policy — unknown errors crash the app. Workers have Degraded — they survive and retry.

### Wire DI — No Magic

All dependencies are wired via Google Wire. Constructors are pure: accept deps, assign fields, return. No side effects, no goroutines in constructors.

## Dive Deeper

- [Two-Phase Scheduler](./two-phase) — how preflights and workers interact
- [Error Handling & Retry](./error-handling) — the full retry pipeline
- [longrun Package](./longrun/index.md) — the standalone task orchestration library
