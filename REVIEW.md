# Review Guidelines

## How to update this document

This is a living document. Update it when:
- A new pattern has proven itself in practice (not just in theory).
- A decision has been made that affects how code is written or reviewed.
- A bug was caused by violating a rule that wasn't documented yet.

How to update:
- Submit changes via PR like any other code. The project author approves.
- Each rule should be grounded in real experience, not hypothetical scenarios.
- Keep it concise. If a rule needs a paragraph of explanation, it's too complex.

## Code style

- **Single responsibility** — small types, small functions, one job each.
- **Open/closed** — extend through new types, don't modify existing ones.
- **Encapsulation** — exported types with unexported fields. Force construction through constructors, not struct literals. Exception: pure config/value types (e.g. `TransientRule`, `Policy`) may use exported fields when validated at construction time by the consuming constructor.
- **Fail early** — invalid configuration panics at construction time, not silently misbehaves at runtime. Panics are for programmer errors (wrong config, nil where not allowed). Errors are for runtime failures (network, IO, external systems).
- **Safe zero-values** — the zero-value of a config field should be safe, not surprising. If zero means "use default", document it in godoc and log a warning so it's visible. Example: `// MaxRetries: 0 (zero-value) → DefaultMaxRetries (3). Logs a warning.`
- **Concrete names** — name types and functions after what they do, not what pattern they follow.
  Banned names: `Service`, `Manager`, `Handler`, `Helper`, `Utils`, `Common`, `Base`, `Processor`, `Coordinator`, `Wrapper`. If you can't name it without a buzzword, the abstraction is wrong. Exception: unexported types implementing a concrete internal contract (e.g. `failureHandler` — handles failures, not "handles requests") are allowed when the name describes the action, not the pattern.
- **`util` is banned** — if something is reusable, extract it into `pkg/` with a semantic package name that describes what it does, not that it's a utility.
- **No broken windows** — a misplaced field, a wrong abstraction level, a "temporary" hack — fix it now, not "later". If one shortcut stays, the next contributor assumes it's the norm and adds another. Codebase quality degrades one tolerated compromise at a time. If you can't fix it in this PR, create an issue immediately — never leave it undocumented.

## Testing

- **Blackbox only** — all tests use `package xxx_test`. No exceptions.
- **If internal logic needs isolated testing, export it** — promote it to a proper type with a clear API. Facade for users, building blocks for testability.
- **Godoc must describe edge cases** — if a function has non-obvious behavior, document it.
- **Packages should have tests** — current coverage is low, this is a known debt.
- **Go examples are encouraged** — `Example*` functions in `_test.go` serve as both documentation and tests.
- **WIP code may skip tests** — application-layer features or unfinished release work don't need tests until stabilized. Infrastructure and `pkg/` code always needs tests.
- **Bug fix = test first** — every bug must be confirmed by a test case before fixing. The test describes the expected (correct) behavior, not the current broken one. Always two separate commits:
  1. Red test that reproduces the bug (expected behavior, currently failing).
  2. Fix that makes the test green.
  Never combine the test and the fix in a single commit — the red state must be observable in history.

## Error handling

- **Sentinel errors at domain boundaries** — define errors where the problem is known (e.g. `ErrFetchIssues` in the parser, not in the HTTP client).
- **Whitelist approach** — if something is not explicitly declared retryable/expected, treat it as fatal.
- **Wrap with sentinel at boundaries** — `fmt.Errorf("%w: description: %w", ErrMySentinel, err)`. The first `%w` is the domain sentinel (catchable via `errors.Is`), the second preserves the original error chain. Without a sentinel, callers can't classify the error.
- **Plain context wrap is fine internally** — `fmt.Errorf("parsing response: %w", err)` is acceptable inside a function when a sentinel will be added higher up the call stack.

## Context

- **Always pass `context.Context`** — first parameter, always. No storing contexts in structs.
- **Always use `...Context()` variants for logging** — `logger.InfoContext(ctx, ...)`, never `logger.Info(...)`. Context carries trace IDs, request scoping, cancellation — losing it means losing observability. Exception: constructors and setup-phase code that run before any request context exists — there is nothing to trace.
- **Never create contexts from scratch** — no `context.Background()` mid-call-chain, ever. Always derive from the parent.
- **Respect parent cancellation** — derive via `context.WithTimeout`, `context.WithCancel`, etc.
- **Graceful shutdown exception** — when cleanup must outlive the parent's cancellation, use `context.WithoutCancel(parent)` + own timeout. Still derived from parent, but cancellation-independent.

## Dependencies

- Minimize external dependencies, especially in `pkg/`.
- `slog` for logging. No third-party loggers.
- Apache 2.0 license header on every `.go` file.

## Architecture

- **Typed constructors over config structs** — when a flat config allows invalid combinations, use separate constructors that make illegal states unrepresentable.
- **Functional options for optional parameters** — required params in the function signature, optional via `With*` functions.
- **Per-entity state** — no shared or global mutable state. Each component owns its data.
- **Pure constructors for Wire** — constructors are auto-generated by Wire and re-generated by IDE. Keep them pure: accept dependencies, assign fields, return. No side effects, no validation, no goroutines. If initialization logic is needed, put it in the single public method of the struct (for single-method types) or use functional options (for types with a broad API). This keeps Wire as a simple wiring tool, not a place where things silently happen.
- **DSL internals may use pragmatic shortcuts** — internal wiring code that runs once at startup may bend the rules (global logger, `any` type switches, panics, unexported wrappers) in service of a clean DSL surface. Complexity stays inside — consumers see only the elegant API.

## Documentation

- **Keep docs in sync** — if a PR changes behavior, update the relevant docs in the same PR. Stale docs are worse than no docs.
- **VitePress is the single source of truth** — all architecture, internals, and contributor docs live in `docs/`. No standalone `README.md` files inside packages — if a package needs documentation, write a VitePress page in `docs/internals/` and register it in `docs/.vitepress/config.ts` sidebar.
- **Devlog is welcome** — architectural decisions, NIH lessons, rollbacks, trade-offs — write it up in `docs/devlog/`. Format: `NNN-slug.md`, register in `docs/.vitepress/config.ts` sidebar.

## Git

Conventional commits. English only.

Format:
```
type(scope): short description

- detail one
- detail two
- detail three
```

Rules:
- **Blank line after header** — always.
- **Body lines are dashes** — short `-` items describing what was done.
- **Every commit must compile** — atomic, conscious, buildable.
- **Linear history** — merge commits are forbidden. Always rebase.
- **Periodically rebase default branch into feature branch** — keep up to date, avoid drift.

### Git reset strategy

When commit history gets messy during development (mixed concerns, WIP noise, interleaved topics), use git reset to restructure it before merge:

1. `git reset --soft <base>` — all changes become staged.
2. `git reset HEAD .` — unstage everything into the working tree.
3. `git add` files per topic, commit in clean sequence — each commit is atomic and compilable.
4. `git push --force-with-lease` — safe force push that won't overwrite others' work.
   **Important:** step 2 is required. After `--soft` reset all changes are staged. Without unstaging first, the first `git add` + `git commit` will capture everything — subsequent commits will have nothing left.

This produces a linear history grouped by topic, not by chronological order of development. **Only do this after the review is approved and all work is done.** Force pushes invalidate existing review comments and make incremental review impossible. Clean up history as the very last step before merge.

## Issue and PR hygiene

- **Labels are mandatory** — every issue and PR must have at least a priority (`P:`), type (`T:`), and area (`A:`) label. Epic labels where applicable.
- **Milestones track releases** — assign issues to the milestone they ship in. An issue without a milestone is invisible to planning.
- **Keep labels current** — if scope shifts during work (e.g. a feature becomes a refactor), update the labels. Stale labels erode trust in the backlog.

## Structural review (Torvalds rigor)

Normal diff review catches local bugs. Structural review catches architectural rot — God Objects, lying names, glossary drift — the kind of damage that compounds silently until the codebase is unmaintainable. Torvalds rejects patches for structural violations even when the code "works". We do the same.

Applies to: any significant refactor, new module, or file that crosses a complexity threshold. Reviewer must check these **before** approving.

### Hard thresholds — block merge

| Metric | Threshold | Action |
|---|---|---|
| File LOC | > 400 | Split before merge |
| Cyclomatic complexity | > 15 | Split or simplify before merge |
| Private methods on one type | > 10 | God Object — extract subsystem |
| Call depth from public entry | > 6 | Flatten or extract intermediate types |

These are not guidelines — they are gates. A file at 410 LOC does not get a pass because "it's close". Split it.

### Naming — no lies in the codebase

- **Function name must describe what it does, not what you wish it did.** A function called `run` that actually restarts a loop is a lie. A method called `Sync` that deletes data is a lie. Lies compound — the next reader trusts the name, builds on wrong assumptions, and ships a bug. Rename before merge.
- **One term, one meaning.** If "retry" means both "re-attempt a failed call" and "restart the entire loop from scratch", pick two different words. If "event" means both "domain event" and "outbox row", disambiguate. A term used with two meanings anywhere in code, comments, or docs must be fixed before merge.

### Structural checks

- **Martin metrics** — track OCP compliance and extension points per module. If adding a new variant (error category, worker type, output format) requires modifying existing code instead of adding a new file, the design is wrong.
- **Glossary consistency** — every domain term must have exactly one meaning across code, comments, and docs. Drift is a bug. If a reviewer spots the same word used for two concepts — even in a comment — it blocks merge.
- **God Object detection** — a type that accumulates unrelated private methods is not "well-encapsulated", it's a junk drawer. Signs: the type has methods that don't share state, or you can draw a line through the method list and get two independent groups. Extract subsystems into separate types or files. The struct stays thin — it delegates, not accumulates.
- **Cognitive load** — if a reviewer needs to hold more than one subsystem in their head to understand a single method, the method is doing too much. If you can't explain a function's control flow in one sentence, split it.

### Origin

These rules were codified after a Torvalds-style structural review (PR #200) caught a God Object, a lying function name, and glossary drift — none of which surfaced in normal diff review. The lesson: diff review is necessary but not sufficient. Structural violations block merge just like a failing test.

## Pull requests

PR description should reflect the overall scope of work in free-form prose. No code examples in the description — that's what the diff is for. Describe *what* and *why*, not *how*.

### Scope

Multiple issues in one PR are allowed, but discipline is required:
- **Atomic compilable commits** — each commit stands on its own.
- **Critical bugs affecting production or public API must be fixed in the same PR** — don't defer what's dangerous.
- **Non-critical bugs and flags → separate issues** — always link back to the current PR. Exception: if the fix is trivial and blocking the merge.
- **Blocked PRs** — if a sub-problem blocks the merge, describe the blocker explicitly. The PR waits.

### Review findings

- **Critical** — fix in this PR before merge.
- **Non-critical** — create an issue, link to PR, move on.
- Resist scope creep. It's fine to notice 10 things. It's not fine to fix all 10 in one PR if they're unrelated.
