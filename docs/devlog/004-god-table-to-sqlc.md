---
title: "autosolve Devlog #4 — From God Table to sqlc"
description: Three database migrations in one project — GORM auto-migrate to goose versioned migrations to sqlc type-safe codegen. Each step fixed a real problem.
---

# #4 — From God Table to sqlc

> Three database migrations in one project. GORM → goose → sqlc. Each step fixed a real problem.

## Phase 1: The God Table (`d35dfe2`)

The first database was a single GORM model. One table, all issue fields crammed together. Auto-migrate on startup. It worked — you could poll issues and store them.

But it was a god table. Every query touched every column. No relations, no foreign keys, no way to track which repository an issue belonged to without stuffing `owner` and `repo` into every row. When multi-repo support appeared on the horizon, this was a dead end.

## Phase 2: Normalized Schema (`3d9c3e7`)

```
3d9c3e7 feat: normalized DB schema — replace god table
         with relational models (#60)
```

Proper relational design: `repositories`, `issues`, `labels`, `users`, `comments` — each with foreign keys. `repository_id` on issues instead of duplicated owner/repo strings.

Still GORM. Still auto-migrate. But the schema was clean.

## Phase 3: Goose Migrations (`ed9e019`)

```
ed9e019 feat(dal): replace GORM AutoMigrate with goose
         versioned migrations (#79)
```

GORM's auto-migrate is convenient until it isn't. It can add columns but can't remove them. It can't do data migrations. It silently ignores things it can't handle. For a daemon that stores persistent state, this is terrifying.

Goose gave us:
- Versioned SQL migrations with up/down
- Explicit control over schema changes
- A `migrate` CLI command with status, redo, rollback

The migration commands (`d49e978`) became first-class citizens: `migrate up`, `migrate down`, `migrate status`, `migrate up:fresh`.

## Phase 4: sqlc (`9607b0c`)

```
9607b0c feat(dal): migrate from GORM to sqlc for type-safe
         query generation
```

GORM was still generating queries at runtime. Reflection-heavy, hard to debug, easy to get wrong. A typo in a struct tag? Silent wrong query.

sqlc flipped the model: **write SQL, generate Go.** You write the actual query in `.sql` files, sqlc generates type-safe Go functions. No reflection, no runtime surprises. The compiler catches mistakes.

```sql
-- name: UpsertIssue :exec
INSERT INTO issues (repository_id, github_id, ...)
VALUES (?, ?, ...)
ON CONFLICT (repository_id, github_id) DO UPDATE SET ...
```

Generates:
```go
func (q *Queries) UpsertIssue(ctx context.Context, arg UpsertIssueParams) error
```

Type-safe. Compile-time checked. No magic.

## The Cleanup (`9a4bea6`, `09beb6f`)

After sqlc landed, a wave of fixes:
- DATETIME typos in schema
- DB defaults instead of application-level defaults
- sqlfluff linting for SQL files
- A custom sqlcgen type safety check in CI
- Graceful DB close with context

## What We Learned

Each migration was painful but necessary:
1. **God table → relational** — forced by multi-repo requirement
2. **Auto-migrate → goose** — forced by needing rollbacks and data migrations
3. **GORM → sqlc** — forced by wanting compile-time safety over runtime reflection

The pattern: **convenience tools are great until your requirements outgrow them.** GORM is perfect for prototypes. sqlc is perfect for production. The trick is knowing when to switch.

::: tip The Rule
From `REVIEW.md`: *"No broken windows — fix it now, not later."* Each database migration was a broken window that got fixed before it spread.
:::

---

*Commits: `d35dfe2` god table → `3d9c3e7` normalized → `ed9e019` goose → `9607b0c` sqlc*
