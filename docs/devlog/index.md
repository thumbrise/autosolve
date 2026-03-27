# Devlog

How we got here. Design decisions, dead ends, and lessons learned — written after the fact, reconstructed from git history and code comments.

::: info Retroactive
We didn't keep a devlog from day one (regret). These entries are reconstructed from commit history, README evolution, and the decisions baked into the code. Some details may be fuzzy, but the reasoning is real.
:::

## Entries

- [#1 — Why Polling, Not Webhooks](/devlog/001-why-polling) — the foundational choice
- [#2 — Graceful Shutdown Is Hard](/devlog/002-graceful-shutdown) — three bugs in a row, all about context
- [#3 — Building longrun](/devlog/003-building-longrun) — why we wrote our own task orchestration package
- [#4 — From God Table to sqlc](/devlog/004-god-table-to-sqlc) — GORM → goose → sqlc, and why each step happened
- [#5 — Two-Phase Scheduler](/devlog/005-two-phase-scheduler) — preflights, multi-repo, and the Planner pattern

---

*Want to write the next one? [Open a PR.](https://github.com/thumbrise/autosolve/edit/main/docs/devlog/)*
