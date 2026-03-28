# #8 — Why We Replaced a Hand-Rolled Job Table with goqite

> Three days building a job lifecycle from scratch. One conversation to realize we shouldn't have.

## The Trigger

```
PR #160 — Jobs table: persistent tracking for AI dispatch results
```

The goal: decouple outbox event processing from AI execution. OutboxRelay reads events, creates pending jobs, a future processor picks them up. Durable queue between detection and action.

## What We Built

A full job lifecycle from scratch:

```sql
CREATE TABLE jobs (
    id, status, type, prompt, model, result, attempts, last_error, ...
);
```

Four status constants. Three Mark methods. Seven SQL queries. A JobRepository with OTEL spans. A domain entity with status/type enums. RetryFailedJobs bulk reset. ListPendingJobs, ListDoneJobs, ListJobsByIssue.

It compiled. It worked. The architecture was clean — entity, repository, migration, sqlc codegen, Wire bindings. Three days of careful work.

## The Question

> "Are we just building stuff that already exists?"

The classic NIH check.

## What We Were Missing

| Concept            | Hand-rolled                                      | Already solved                            |
|--------------------|--------------------------------------------------|-------------------------------------------|
| Status lifecycle   | 4 constants + 3 SQL queries                      | Built into any job queue                  |
| Visibility timeout | **Missing** — crashed worker = stuck job forever | Automatic redelivery                      |
| Atomic dequeue     | `MarkProcessing` didn't check `RowsAffected`     | `SELECT ... UPDATE RETURNING` in one shot |
| Retry with backoff | `RetryFailedJobs` — bulk reset, no backoff       | Exponential backoff per message           |
| Dead letter        | Manual `attempts < ?`                            | Built-in max receive count                |

The killer was **visibility timeout**. If the process crashed after `MarkProcessing` but before `MarkDone`, the job was stuck in `processing` forever. No amount of `RetryFailedJobs` would fix it — that only resets `failed` jobs, not `processing` ones. We would have needed a separate reaper goroutine. More code. More bugs. More NIH.

## The Choice: goqite

[goqite](https://github.com/maragudk/goqite) — Go + SQLite + queue. ~300 lines. One table. Zero external dependencies beyond SQLite.

Why it fits:

1. **SQLite-native** — our only storage. No Redis, no Postgres, no broker.
2. **Visibility timeout** — message reappears automatically if consumer dies.
3. **Atomic dequeue** — `UPDATE ... RETURNING` in a transaction. No race conditions.
4. **Max receive** — dead messages stop redelivering. No infinite loops.
5. **Postgres support** — `SQLFlavorPostgreSQL` flag for when we migrate post-MVP.

## What Changed

```
// Before:
OutboxRelay → jobRepo.Create() → jobs table (hand-rolled lifecycle)

// After:
OutboxRelay → queue.Send() → goqite table (battle-tested lifecycle)
```

Domain stays clean — `OutboxRelay` depends on a `JobQueue` interface with primitive arguments:

```go
type JobQueue interface {
    Send(ctx context.Context, jobType string, repositoryID, issueID int64) error
}
```

Infrastructure implements it with JSON serialization into goqite. Domain doesn't know goqite exists.

## What We Deleted

- `004_create_jobs.sql` → replaced with goqite schema
- `jobs.sql` (7 queries) → gone
- `jobs.sql.go` (283 lines generated) → gone
- `JobRepository` (118 lines) → gone
- `Job` entity (37 lines) → gone
- `GlobalWorkerSpec` → gone (goqite's `jobs.Runner` covers this). *Brought back in #156 as a minimal spec for non-repo-scoped workers. Full refactor to `TaskSpec[T]` deferred to #161.*

Total: ~500 lines of hand-rolled code replaced by a 90-line wrapper over a proven library.

## What We Kept

- **OutboxRelay** — same concept, just calls `queue.Send()` instead of `jobRepo.Create()`
- **Outbox pattern** — `IssueSyncer` → `outbox_events` → `OutboxRelay` → untouched
- **`GetIssueByRepoAndNumber` with `id`** — still needed for the job payload

## The Lesson

**NIH is not about pride — it's about awareness.** We didn't build a job table because we thought we were smarter than existing solutions. We built it because we didn't stop to ask "has someone already solved this for SQLite?"

The answer was yes. And their solution handles edge cases we hadn't even discovered yet.

> **Use libraries for lifecycle. Write code for domain.**

The job queue is plumbing. goqite does plumbing. Our code should do the thing that's unique to autosolve — deciding *what* to enqueue and *what* to do with the result.

---

*PR: #160 — the one where we deleted 500 lines and got a better system.*
