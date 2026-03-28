# autosolve

[![CI](https://github.com/thumbrise/autosolve/actions/workflows/ci.yml/badge.svg)](https://github.com/thumbrise/autosolve/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/thumbrise/autosolve.svg)](https://pkg.go.dev/github.com/thumbrise/autosolve)
[![GitHub stars](https://img.shields.io/github/stars/thumbrise/autosolve?style=social)](https://github.com/thumbrise/autosolve/stargazers)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](/LICENSE)

Self-hosted Go daemon that polls GitHub repositories and dispatches AI agents to analyze issues automatically. No webhooks, no CI glue — just run and forget.

> **🚧 Active development — beta.** The core pipeline works end-to-end: issues are polled, analyzed by a local LLM, and results are posted as GitHub comments. [See it in action.](https://github.com/thumbrise/autosolve/pull/179) [Contributions welcome.](https://thumbrise.github.io/autosolve/contributing/adding-worker)

## How it works

1. **Polls** your GitHub repositories for new and updated issues
2. **Sends** each issue to a local Ollama model for AI analysis
3. **Posts** the result as a comment on the GitHub issue — automatically

> 🔗 **[Real example — AI analysis posted on a live issue](https://github.com/thumbrise/autosolve/pull/179)**

## Quick Start

```bash
git clone https://github.com/thumbrise/autosolve.git && cd autosolve
go mod download
cp config.yml.example config.yml   # set your token + repos
go run . migrate up -y
go run . schedule
```

Configure in `config.yml`:
```yaml
github:
  token: ghp_your_token          # needs issues:write scope
  repositories:
    - owner: your-org
      name: your-repo
  issues:
    requiredLabel: "autosolve"   # optional — only analyze labeled issues

ollama:
  endpoint: "http://localhost:11434"
  model: "qwen2.5-coder:7b"     # any Ollama model
```

That's it. Every issue with the `autosolve` label gets an AI analysis comment within seconds.

## Documentation

📖 **[thumbrise.github.io/autosolve](https://thumbrise.github.io/autosolve)** — full docs, guides, architecture, devlog.

| Section | What's there |
|---------|-------------|
| [Quick Start](https://thumbrise.github.io/autosolve/guide/getting-started) | Setup in 5 minutes |
| [Configuration](https://thumbrise.github.io/autosolve/guide/configuration) | All config options |
| [Architecture](https://thumbrise.github.io/autosolve/internals/overview) | How the system works |
| [The Idea](https://thumbrise.github.io/autosolve/project/idea) | Why this project exists |
| [Contributing](https://thumbrise.github.io/autosolve/contributing/adding-worker) | Add a worker in 4 steps |
| [Devlog](https://thumbrise.github.io/autosolve/devlog/) | How we got here — design decisions diary |

## Current Status

Epic v1 is in progress — see [Epic: v1 architecture redesign](https://github.com/thumbrise/autosolve/issues/59).

What works today: end-to-end AI dispatch pipeline — multi-repo polling, issue sync, outbox relay, Ollama analysis, GitHub comment posting, feedback loop prevention, per-error retry with degraded mode, rate limiting, full OTEL observability, SQLite with goose + sqlc.

What's next: re-analysis on issue updates, adaptive polling, CLI commands, GitHub App migration.

## License

[Apache License 2.0](LICENSE)