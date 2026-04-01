---
title: "resilience: Options & Plugins"
description: Two extension points — Option for per-call behavior, Plugin for shared lifecycle. How to use, when to choose which, how to write your own.
---

# Options & Plugins

resilience has two extension points. They look similar but serve different purposes.

## Option — per-call behavior

```go
type Option func(ctx context.Context, call func(context.Context) error) error
```

An Option wraps a function call. It receives the context and the next call in the chain. It controls execution — retry, timeout, rate-limit, or anything else.

Options are fresh on every `Do()`. No shared state. No data races.

### Using Options

```go
client.Call(fn).
    With(retry.On(ErrTimeout, 3, bo)).
    With(timeout.After(5*time.Second)).
    Do(ctx)
```

Options wrap in order: first option is outermost. In the example above, timeout wraps retry wraps fn.

### Writing a simple Option

```go
func Timeout(d time.Duration) resilience.Option {
    return func(ctx context.Context, call func(context.Context) error) error {
        ctx, cancel := context.WithTimeout(ctx, d)
        defer cancel()
        return call(ctx)
    }
}
```

### Writing an advanced Option

Options can access plugin Events via context for observability:

```go
func MyRetry(maxAttempts int) resilience.Option {
    return func(ctx context.Context, call func(context.Context) error) error {
        events := resilience.EventsFromContext(ctx)
        for attempt := 0; attempt < maxAttempts; attempt++ {
            err := call(ctx)
            if err == nil {
                return nil
            }
            resilience.EmitBeforeWait(ctx, events, "my-retry", attempt, 1*time.Second)
            resilience.SleepCtx(ctx, 1*time.Second)
        }
        return call(ctx)
    }
}
```

## Plugin — shared lifecycle

```go
type Plugin interface {
    Name() string
    Events() Events
}
```

A Plugin lives on the Client. Its Events hooks fire on every call made through that Client. Plugins observe — they don't control execution flow.

### Using Plugins

```go
// Client-level — shared across all calls
client := resilience.NewClient(
    rsotel.Plugin(),
    myLoggingPlugin,
)

// Call-level — scoped to this call
client.Call(fn).
    WithPlugin(requestScopedLogger).
    With(retry.On(err, 3, bo)).
    Do(ctx)
```

### Writing a Plugin

```go
type loggingPlugin struct {
    logger *slog.Logger
}

func (p *loggingPlugin) Name() string { return "logging" }

func (p *loggingPlugin) Events() resilience.Events {
    return resilience.Events{
        OnAfterCall: func(ctx context.Context, attempt int, err error, d time.Duration) {
            if err != nil {
                p.logger.ErrorContext(ctx, "call failed",
                    slog.Int("attempt", attempt),
                    slog.Any("error", err),
                )
            }
        },
    }
}
```

## When to use which

| Question | Option | Plugin |
|----------|--------|--------|
| Does it need to control execution? | ✅ | ❌ |
| Does it have shared state across calls? | ❌ | ✅ |
| Is it per-call? | ✅ | Either |
| Does it observe without affecting flow? | ❌ | ✅ |

**Rule of thumb:** if it wraps `call()` — Option. If it watches from the side — Plugin.
