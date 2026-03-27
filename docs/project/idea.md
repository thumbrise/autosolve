# The Idea

::: tip TL;DR
AI dev tools are powerful but integrating them with GitHub is painful. autosolve is a simple self-hosted bridge: **poll → detect → dispatch → report**.
:::

## The Problem

Tools like OpenHands, SWE-agent, Devin can write code. But connecting them to your GitHub workflow is surprisingly hard:

- Webhooks need infrastructure and break silently
- GitHub Actions cron is unreliable for long-running agents
- Most tools are designed for one-off runs, not continuous monitoring
- For small teams, the setup overhead kills the value

## The Solution

A lightweight daemon that:

1. **Polls** GitHub API on a schedule (no webhooks needed)
2. **Detects** new issues, PRs, comments, labels matching your rules
3. **Dispatches** any AI tool you want (ra-aid, Ollama, Python scripts, anything CLI-callable)
4. **Reports** results back to GitHub — comments, PRs, status updates

All state is persisted locally. Missed nothing after restarts. Runs on a laptop or a Raspberry Pi.

## Why Self-Hosted

- Your tokens stay on your machine
- No third-party services in the loop
- Works with private repos without exposing anything
- You control the AI tool, the rules, and the budget

## Where We Are

The project is in **active development**. The core polling and scheduling infrastructure works. The AI dispatch layer is next.

Read the [full original idea document](https://github.com/thumbrise/autosolve/blob/main/docs/IDEA.md) for the complete vision, or check [Status & Roadmap](./status) for what's done and what's planned.

::: info Want to help shape this?
The architecture is still forming. Now is the best time to influence the direction. [Open an issue](https://github.com/thumbrise/autosolve/issues/new) or check the [Ideas & Wishlist](./ideas).
:::
