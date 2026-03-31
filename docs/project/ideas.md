---
title: autosolve Ideas & Wishlist
description: Open list of future ideas for autosolve — plugin system, rule engine for AI dispatch, multi-agent support, adaptive polling, and more.
---

# Ideas & Wishlist

::: tip This is an open list
Have an idea? [Open an issue](https://github.com/thumbrise/autosolve/issues/new) or submit a PR adding it here. Nothing is too wild at this stage.
:::

## Big Ideas

These would change how the project works fundamentally.

### Plugin / Module System
Each partition type becomes a self-contained module with its own preflights, workers, and DAL. Register modules explicitly:
```go
modules := []Module{repo.New(), analytics.New(), notifications.New()}
```
Same pattern as Linux loadable modules or PHP extensions. See [internal/application/README.md](https://github.com/thumbrise/autosolve/blob/main/internal/application/README.md#future-module-system) for the design sketch.

### Rule Engine for AI Dispatch
Configurable rules that decide when to launch an AI agent:
- Issue has label `ai`
- Comment contains `@bot`
- PR has no reviewers after 24h
- New issue matches a pattern

### Multi-Agent Support
Different AI tools for different tasks. ra-aid for code fixes, a custom script for triage, Ollama for analysis. The executor layer should be pluggable.

### Org-Level Partitions
Today the unit of work is a repository. But some tasks make sense at the org level — cross-repo analytics, org-wide triage, dependency scanning.

## Smaller Ideas

Things that would be nice to have.

- **Adaptive polling** — back off when repos are quiet, speed up when active ([#53](https://github.com/thumbrise/autosolve/issues/53))
- **Comment polling** — watch for `@bot` mentions, not just issues
- **PR polling** — track pull requests, not just issues
- **Webhook mode** — optional webhook receiver for instant reaction (polling as fallback)
- **Web dashboard** — simple status page showing what the daemon is doing
- **Docker image** — official image for easy deployment
- **Notification channels** — Slack, Telegram, Discord when agent completes a task
- **Dry-run mode** — show what the agent *would* do without actually doing it
- **Cost tracking** — track API calls and AI token usage per repo

## longrun Package Ideas

The `pkg/longrun` package is designed for extraction into a standalone module. Ideas for it:

- **DependsOn** — `runner.Add(task, longrun.DependsOn("migrate"))` for task ordering
- **Circuit breaker** per TransientRule — open/half-open/closed state machine
- **Health endpoint** — expose task status via HTTP for external monitoring
- **Metrics dashboard** — Grafana template for longrun metrics

See the [longrun Roadmap](../internals/longrun/roadmap.md) for the package-specific roadmap.

---

*This list is intentionally broad. Not everything here will be built, and that's fine. The point is to capture directions and invite discussion.*
