# autosolve

[![CI](https://github.com/thumbrise/autosolve/actions/workflows/ci.yml/badge.svg)](https://github.com/thumbrise/autosolve/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/thumbrise/autosolve.svg)](https://pkg.go.dev/github.com/thumbrise/autosolve)
[![GitHub stars](https://img.shields.io/github/stars/thumbrise/autosolve?style=social)](https://github.com/thumbrise/autosolve/stargazers)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](/LICENSE)

Available languages: [English](README.md), [Русский](README.ru.md)

Self-hosted Go daemon that polls GitHub repositories and dispatches AI agents to solve issues automatically. No webhooks, no CI glue — just run and forget.

> **🚧 Active development.** The core infrastructure works. AI dispatch layer is next. [Contributions welcome.](https://thumbrise.github.io/autosolve/contributing/adding-worker)

## Quick Start

```bash
git clone https://github.com/thumbrise/autosolve.git && cd autosolve
go mod download
cp config.yml.example config.yml   # set your token + repos
go run . migrate up -y
go run . schedule
```

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

What works today: multi-repo polling, two-phase scheduler, per-error retry with degraded mode, rate limiting, full OTEL observability, SQLite with goose + sqlc.

What's next: AI dispatch rule engine, result publishing back to GitHub, adaptive polling.

## License

[Apache License 2.0](LICENSE)