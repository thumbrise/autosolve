---
title: Getting Started with autosolve
description: Install and run autosolve in minutes — clone the repo, configure your GitHub token, migrate the database, and start the polling daemon.
---

# Getting Started

::: warning Project Status
autosolve is in active development. Things work, but APIs may change. See [Status & Roadmap](/project/status).
:::

## Prerequisites

- Go 1.26+
- GitHub personal access token with repo access

## Clone → Configure → Run

```bash
git clone https://github.com/thumbrise/autosolve.git
cd autosolve
go mod download
cp config.yml.example config.yml
```

Edit `config.yml` — set your token and repos:

```yaml
github:
  token: ghp_your_token_here
  repositories:
    - owner: your-org
      name: your-repo
```

Migrate and start:

```bash
go run . migrate up -y
go run . schedule
```

That's it. The daemon will validate your repos (preflights), then start polling issues on the configured interval.

::: tip Using Task
If you have [Task](https://taskfile.dev) installed: `task up` does migrate + schedule in one command.
:::

## What Happens on Start

1. **Preflights** — validates that every configured repo is accessible via GitHub API, upserts repo records into SQLite
2. **Workers** — starts polling issues for each repo on the configured interval (default 5s)

If any preflight fails, workers never start. This is by design — see [Two-Phase Scheduler](/internals/two-phase).

## Commands

| Command | What it does |
|---------|-------------|
| `schedule` | Start the daemon |
| `migrate up [-y]` | Apply migrations |
| `migrate up:fresh` | Drop everything, re-migrate |
| `migrate down <N\|*>` | Roll back |
| `migrate status` | Show migration state |
| `migrate create <name>` | New migration file |
| `jobs list` | Show all pending messages in the queue |
| `jobs show <id>` | Show one message with full payload |

## Next

- [Configuration](./configuration) — all config options
- [Observability](./observability) — OpenTelemetry setup
- [Architecture](/internals/overview) — how the system works under the hood
