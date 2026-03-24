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
- **Encapsulation** — exported types with unexported fields. Force construction through constructors, not struct literals. Exception: pure config/value types (e.g. `BackoffConfig`, `TransientRule`) may use exported fields when validated at construction time by the consuming constructor.
- **Fail early** — invalid configuration panics at construction time, not silently misbehaves at runtime. Panics are for programmer errors (wrong config, nil where not allowed). Errors are for runtime failures (network, IO, external systems).
- **Safe zero-values** — the zero-value of a config field should be safe, not surprising. If zero means "use default", document it in godoc and log a warning so it's visible. Example: `// MaxRetries: 0 (zero-value) → DefaultMaxRetries (3). Logs a warning.`
- **Concrete names** — name types and functions after what they do, not what pattern they follow.
  Banned names: `Service`, `Manager`, `Handler`, `Helper`, `Utils`, `Common`, `Base`, `Processor`, `Coordinator`, `Wrapper`. If you can't name it without a buzzword, the abstraction is wrong.
- **`util` is banned** — if something is reusable, extract it into `pkg/` with a semantic package name that describes what it does, not that it's a utility.

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
- **Always use `...Context()` variants for logging** — `logger.InfoContext(ctx, ...)`, never `logger.Info(...)`. Context carries trace IDs, request scoping, cancellation — losing it means losing observability.
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
