# #6 — How I Over-Engineered an Event Bus and Rolled Back
> From generic `Topic[TTopic, TEvent, TOffset]` to a simple `Save + Cursor` interface. The commit that almost wasn't.
## The Trigger
```
PR #141 — Atomic issue sync with transactional outbox
```
The goal was simple: make issue sync atomic. Upsert issues + save cursor + write outbox event — one transaction. The outbox pattern. Textbook stuff.
But I didn't stop there.
## The Spiral
It started innocently. "If we're writing outbox events, we'll need a consumer eventually. Let's define the contract now."
So I built a generic event bus interface:
```go
type Topic[TTopic any, TEvent any, TOffset any] interface {
    Produce(ctx context.Context, topic TTopic, repositoryID int64, events []TEvent, next TOffset) error
    Offset(ctx context.Context, topic TTopic, repositoryID int64) (TOffset, error)
    Consume(ctx context.Context, topic TTopic, limit int) ([]TEvent, error)
    Ack(ctx context.Context, event TEvent) error
}
```
Three type parameters. Four methods. Implementations could be SQLite outbox, Redis streams, Kafka, memory — anything. Beautiful.
Then `IssueSyncer` had to implement it. Which meant `Produce` needed to upsert issues AND save cursor AND write outbox events. The event type `IssueEvent` grew an `Issue *Issue` field so the DAL could extract the entity from the event. The domain was shaping itself around the infrastructure contract.
The poller wrapped every issue in an `IssueEvent` just to pass it through `Produce`. `buildEvents()` existed solely to satisfy the interface. The code compiled. It worked. The logs looked great.
But reading it back — it was a God Object wearing a generic trench coat.
## The Realization
Staring at `IssueSyncer` with fresh eyes:
1. **`Produce` knew about Issue entities** — 14-field upsert inside an "event bus" method. An event bus should not know what an Issue looks like.
2. **`IssueEvent.Issue *Issue`** — the domain event carried a full entity as payload, purely so the DAL could persist it. The event was a transport mechanism for upsert data. That's not what events are for.
3. **`Consume` + `Ack` had no callers** — written for a future consumer that didn't exist. YAGNI in its purest form.
4. **The poller was wrapping and unwrapping** — `[]*Issue` → `[]IssueEvent` → extract `Issue` back out in DAL. A round trip that added nothing.
   The generic contract was correct in isolation. But the implementation collapsed four responsibilities into one struct: aggregate root, outbox writer, event consumer, and event acknowledger.
## The Rollback
Deleted everything that wasn't needed today:
- `Topic[TTopic, TEvent, TOffset]` — gone. Returns with the broker PR.
- `IssueEvent` — gone. Returns when there's an actual consumer.
- `Consume`, `Ack` — gone. No caller, no code.
- `Offset` renamed to `Cursor` — it's a poll cursor, not a broker offset.
  What remained:
```go
type IssueSyncRepo interface {
    Save(ctx context.Context, repositoryID int64, issues []*entities.Issue, cursor entities.Cursor) error
    Cursor(ctx context.Context, repositoryID int64) (entities.Cursor, error)
}
```
Two methods. No generics. No type parameters. The poller calls `Cursor()` to know where it left off, calls GitHub, calls `Save()` with issues and the next cursor. Inside `Save()` — one transaction: upsert + cursor + outbox row. The outbox is an implementation detail the poller doesn't know about.
## What Stayed
The outbox table and migration survived. They're the foundation for the dispatch pipeline. The aggregate root writes outbox rows atomically. A future relay worker will read them and push to whatever broker we choose. But that's a different PR with a different interface.

## What We Learned
**The outbox pattern is about atomicity, not about event bus contracts.** The transaction boundary is the value. Everything else — consumers, topics, offsets, ack — belongs to the broker layer that doesn't exist yet.
**Generic interfaces earn their keep when they have two implementations.** With one implementation and zero consumers, `Topic[TTopic, TEvent, TOffset]` was a liability, not an asset.
**Rolling back is harder than building.** Deleting code you spent hours on feels wrong. But shipping a God Object "because it works" would have poisoned every future PR. The architecture would have calcified around the wrong abstraction.

---
*PR: #141 — the spiral and the rollback, all in one.*