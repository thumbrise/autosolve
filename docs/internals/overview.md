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
Scheduler
  ├── Phase 1: Preflights (one-shot, all must pass)
  └── Phase 2: Workers
        ├── Per-repo workers (multiplied by Planner)
        └── Global workers (shared resources, not per-repo)
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
│   └── spec/           Task specs + tenants
│       ├── preflights/ RepositoryValidator
│       ├── workers/    IssuePoller, OutboxRelay, IssueExplainer
│       └── tenants/    RepoTenant
├── application/        Orchestration layer
│   ├── schedule.go     Two-phase Scheduler
│   ├── planner.go      Per-repo task planning
│   ├── contracts.go    Preflight / Worker interfaces
│   └── registry.go     Task registration
└── infrastructure/     External dependencies
    ├── github/         GitHub API client + rate limiter
    ├── dal/            Data access (sqlc-generated)
    ├── database/       SQLite + goose migrations
    └── telemetry/      OTEL SDK bootstrap
pkg/
└── longrun/            Task orchestration (standalone package)
```

## Key Design Decisions

### Domain Doesn't Know About Retry

Domain types (`RepositoryValidator`, `IssuePoller`) declare *what* to do via specs. They don't know about retry, backoff, or error classification. That's all handled by `longrun.Runner` through Baseline policies configured by `Scheduler`.

### Planner Owns the Multi-Repo Concept

`Planner` takes domain specs and multiplies them by configured repositories. Each repo gets its own closure with a captured `RepoTenant`. Domain code receives a tenant and does its job — it never knows how many repos exist.

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
- [longrun Package](./longrun) — the standalone task orchestration library
