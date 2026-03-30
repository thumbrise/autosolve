---
title: "autosolve Devlog #10 — The Partition Refactor"
description: How three spec types became two, why we stopped injecting tenants through context, and what MapReduce has to do with a GitHub polling daemon.
---

# #10 — The Partition Refactor

> Domain should say what it needs, not dig through context to find it.

## The Trigger

Three spec types. Three contract interfaces. Three registry functions. Adding a worker meant knowing which of the three paths to use. Planner knew about repositories. Scheduler knew about global workers. Knowledge was smeared across layers like butter on too much bread.

## What We Tried

First attempt: generics. `TaskSpec[T]` parameterized by tenant type. Elegant in theory, but Go generics don't erase — you can't put `TaskSpec[RepoTenant]` and `TaskSpec[struct{}]` in the same slice. Planner needs one slice. Dead end.

Second attempt: `Tenant` + `Group` + `Bind`. Higher-order functions, context injection. Domain calls `tenants.Repository(ctx)` to get its data. It worked. It ran. But every spec had this line:

```go
return tenants.Repository(ctx) // where did this come from?
```

Context injection is magic. Magic is technical debt with good marketing.

## The Insight

A poller that needs a repository should say so in its function signature:

```go
func (p *IssuePoller) Run(ctx context.Context, partition repository.Partition) error
```

Not hide it inside context. The function signature *is* the documentation. If you need a partition, take a partition.

This meant domain specs couldn't have a uniform `func(ctx) error` signature anymore. But that's application's problem — domain declares what it needs, application wraps it.

## Tenant → Partition

We renamed `Tenant` to `Partition` halfway through. The word "tenant" implies multi-tenancy SaaS. What we actually have is data partitioning — work divided by repository. Each configured repo is a partition. Tasks are multiplied by partitions. The number of goroutines equals the number of partitions.

This naming shift unlocked a bigger insight: partition-bound tasks and queue consumers scale along **orthogonal axes**. Pollers scale by adding repos (more partitions). The explainer scales by adding concurrency (more consumers on the same queue). These are independent. Like MapReduce — Map scales by data, Reduce scales by compute.

## The DSL

The registry reads like a table of contents:

```go
repos.Pack(
    Preflight(repoValidator.TaskSpec()),
    issuePoller.TaskSpec(),
    outboxRelay.TaskSpec(),
),
globalTasks(
    issueExplainer.TaskSpec(),
),
```

`Pack()` multiplies. `Preflight()` marks phase. `globalTasks()` wraps. Domain doesn't know about any of this. It declares three fields — `Resource`, `Interval`, `Work` — and goes home.

Adding a task: one line. Adding a partition dimension (e.g. organizations): one new provider, one new registry section. Planner and Scheduler untouched.

## What Got Deleted

- `PreflightSpec`, `WorkerSpec`, `GlobalWorkerSpec` — three types → two (`TaskSpec`, `GlobalTaskSpec`)
- `Preflight`, `Worker`, `GlobalWorker` interfaces — gone entirely
- `NewPreflights()`, `NewWorkers()`, `NewGlobalWorkers()` — three functions → one (`NewTasks`)
- `contracts.go` — deleted
- `tenants/global.go` — deleted
- Context injection (`tenants.Repository(ctx)`) — deleted
- `preflights/` and `workers/` packages — replaced by `repository/` and `global/` bounded contexts

## What We Learned

Honest signatures beat clever injection. When a function declares its dependencies in the signature, you can read the code without running it. When it pulls things from context, you need a debugger and tribal knowledge.

The DSL internals (`Pack`, `Preflight`, type switch on `any`) are ugly. That's fine. Ugly internals in service of a clean surface is a good trade. The consumer sees `repos.Pack(poller.TaskSpec())`. The maintainer sees closures and type switches. One of them matters more.

::: tip Hindsight
We almost added generics for `TaskSpec[T]`. Glad we didn't — the copy-paste cost of one struct per partition type is ~30 lines. Generics would have saved those lines but added cognitive load to every contributor who reads the code. When N > 2 partition types, we'll reconsider.
:::

---

*PR: #194 — closes #161*
