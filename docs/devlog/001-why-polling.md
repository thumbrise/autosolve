---
title: "autosolve Devlog #1 — Why Polling"
description: Why autosolve uses polling instead of webhooks — no public endpoint, crash-safe cursor pattern, deployable anywhere from a laptop to a Raspberry Pi.
---

# #1 — Why Polling, Not Webhooks

> The first architectural decision. Everything else follows from this.

## The Question

When you want to react to GitHub events, there are two obvious approaches:

1. **Webhooks** — GitHub pushes events to your endpoint
2. **Polling** — you periodically ask GitHub "what's new?"

Every serious integration uses webhooks. It's real-time, it's efficient, it's the "right" way. So why did we choose polling?

## The Problem with Webhooks

Webhooks need infrastructure:

- A public URL (or ngrok, or a tunnel)
- SSL certificates
- Signature validation
- Retry handling for missed deliveries
- A web server that's always running and reachable

For a tool that's supposed to run on a laptop or a Raspberry Pi, that's a lot of overhead. The whole point of autosolve is "just run and forget." Webhooks are the opposite of that — they're "set up infrastructure and maintain it."

## Why Polling Works

Polling is dumb, and that's its strength:

- **No public endpoint** — runs behind any firewall, NAT, VPN
- **No infrastructure** — no DNS, no SSL, no reverse proxy
- **Crash-safe** — missed a cycle? Next one catches up. State is in SQLite.
- **Deployable anywhere** — laptop, server, Raspberry Pi, Docker, systemd

The trade-off is latency. Webhooks react in seconds, polling reacts in minutes. For AI agent dispatch, minutes is fine. Nobody expects an AI to fix their bug in real-time.

## The Cursor Pattern

To make polling reliable, we use a **sync cursor** — a bookmark that tracks where we left off:

- `since` timestamp — only fetch issues updated after this point
- `page` number — for paginating through large result sets
- `ETag` — GitHub's conditional request header, returns 304 when nothing changed

The cursor is persisted in SQLite per repository. After a restart, polling resumes exactly where it stopped. No duplicate processing, no missed events.

## What We Learned

Polling is unsexy but operationally simple. The complexity budget we saved on infrastructure went into making the polling loop robust — retry, backoff, rate limiting, cursor persistence. That turned out to be a much better investment.

::: tip Hindsight
We might add an optional webhook receiver later (it's on the [Ideas list](/project/ideas)). But polling will always be the default — it's the zero-config path.
:::

---

*Commits: `0110a08` init → `65dcbf0` first issue polling*
