# longrun — TODO

## Battle-test in production
The package is currently used inside `autosolve` as the only consumer.
Run it in production, collect real-world feedback, stabilize the API.
Only after the API proves itself stable — extract into a standalone repository.

## Extraction into a standalone Go module
The package is designed to have zero internal dependencies (only `golang.org/x/sync`).
Extract into a separate repository with its own `go.mod` once the API is stable.
Breaking changes before extraction are cheap; after — they require a major version bump.

## Shutdown ordering
Currently shutdown hooks run in LIFO order (reverse of Add).
When DependsOn is implemented, shutdown must follow reverse topological order of the dependency DAG.
LIFO is a correct special case of a linear dependency chain, so the transition should be non-breaking.

## DependsOn
`runner.Add(task, longrun.DependsOn("migrate"))` — Runner-level dependency declaration.
Task knows *how* to work, Runner knows *when* to start.
Requires building a DAG at Runner.Wait time and starting tasks in topological order.

## Circuit breaker per TransientRule
Open/half-open/closed state machine per rule.
When a rule trips the breaker, errors matching that rule become immediately permanent
until the half-open probe succeeds.
