// Copyright 2026 thumbrise
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package resilience provides composable resilience for function calls.
//
// Two levels of configuration:
//
// [Client] — application-wide instance with [Plugin]s (OTEL, circuit breaker).
// Plugins have shared state and observe all calls via lifecycle [Events].
// Create once, pass everywhere — like http.Client.
//
//	client := resilience.NewClient(rsotel.Plugin())
//
// [CallBuilder] — per-call configuration with [Option]s (retry, timeout).
// Options wrap execution — each call gets fresh instances, no shared state.
//
//	err := client.Call(fn).
//	    With(retry.On(ErrTimeout, 3, backoff.Exponential(1*time.Second, 30*time.Second))).
//	    Do(ctx)
//
// [Do] — stateless shortcut for one-off calls without a Client:
//
//	err := resilience.Do(ctx, fn,
//	    retry.On(ErrTimeout, 3, backoff.Exponential(1*time.Second, 30*time.Second)),
//	)
//
// Extension points:
//   - [Option] — func(ctx, call) error. Full control over execution. Per-call.
//   - [Plugin] — interface with lifecycle. Shared state. Client-level.
//   - [Preset] — tested combination of Options with metadata for introspection.
package resilience

import (
	"context"
	"time"
)

// Option wraps a function call with resilience behavior.
// It receives the context and the next call in the chain, and controls
// execution: retry, timeout, rate-limit, or anything else.
//
// Options are the universal extension point. Sub-packages (retry, timeout)
// and community plugins return Options. Users compose them via [CallBuilder.With].
//
// Simple options use helpers from sub-packages.
// Advanced options get full control over the call:
//
//	func MyOption() resilience.Option {
//	    return func(ctx context.Context, call func(context.Context) error) error {
//	        // before call
//	        err := call(ctx)
//	        // after call
//	        return err
//	    }
//	}
type Option func(ctx context.Context, call func(context.Context) error) error

// Plugin is a client-level lifecycle block with shared state.
// Plugins observe all calls via [Events] hooks without affecting control flow.
//
// Register on [Client] via [NewClient]. Plugin state lives for the lifetime
// of the Client — use for cross-call concerns: metrics, circuit breakers, logging.
//
// Implementations: rsotel.Plugin, circuit.Plugin.
type Plugin interface {
	// Name returns the plugin identifier (e.g. "otel", "circuit:github").
	Name() string

	// Events returns lifecycle hooks for this plugin.
	// Called once at Client construction. All fields are optional.
	Events() Events
}

// Events receives lifecycle notifications from the pipeline.
// All fields are optional — nil callbacks are skipped.
// Plugins return Events from [Plugin.Events] to observe without affecting control flow.
type Events struct {
	// OnBeforeCall is called before each fn invocation (including retries).
	OnBeforeCall func(ctx context.Context, attempt int)

	// OnAfterCall is called after each fn invocation with the result.
	OnAfterCall func(ctx context.Context, attempt int, err error, duration time.Duration)

	// OnBeforeWait is called before sleeping (e.g. retry backoff).
	OnBeforeWait func(ctx context.Context, option string, attempt int, wait time.Duration)
}

// Client is an application-wide resilience instance with plugins.
// Immutable after creation. Thread-safe. One per application, pass everywhere.
//
// Plugins provide shared state (circuit breakers) and observability (OTEL).
// Options are per-call — see [CallBuilder].
//
//	client := resilience.NewClient(rsotel.Plugin())
type Client struct {
	plugins []Plugin
	events  []Events
}

// NewClient creates a Client with the given plugins.
//
//	client := resilience.NewClient(
//	    rsotel.Plugin(),
//	    circuit.Plugin("github", circuit.Threshold(10)),
//	)
func NewClient(plugins ...Plugin) *Client {
	events := make([]Events, len(plugins))
	for i, p := range plugins {
		events[i] = p.Events()
	}

	return &Client{plugins: plugins, events: events}
}

// Call starts building a resilient call. Returns a [CallBuilder] — no execution yet.
//
//	client.Call(fn).With(retry.On(...)).Do(ctx)
func (c *Client) Call(fn func(context.Context) error) *CallBuilder {
	return &CallBuilder{fn: fn, client: c}
}

// CallBuilder configures a single resilient call.
// Options and plugins are per-call — each [Do] is independent.
type CallBuilder struct {
	fn      func(context.Context) error
	client  *Client
	options []Option
	plugins []Plugin
}

// With adds per-call options. Options wrap execution in order:
// first option is the outermost wrapper.
//
//	builder.With(timeout.After(5*time.Second), retry.On(err, 3, bo))
//	// timeout wraps retry wraps fn
func (b *CallBuilder) With(opts ...Option) *CallBuilder {
	b.options = append(b.options, opts...)

	return b
}

// WithPlugin adds per-call plugins. Same [Plugin] interface as client-level,
// but state is scoped to this call.
func (b *CallBuilder) WithPlugin(plugins ...Plugin) *CallBuilder {
	b.plugins = append(b.plugins, plugins...)

	return b
}

// Do executes the call with all configured options and plugins.
// Options are applied as a middleware chain: first option is outermost.
// Plugin events are available to options via [EventsFromContext].
func (b *CallBuilder) Do(ctx context.Context) error {
	events := b.mergeEvents()

	// Attach events to context so options can emit lifecycle notifications.
	if len(events) > 0 {
		ctx = withEvents(ctx, events)
	}

	// Build the middleware chain: wrap fn with options, last option innermost.
	call := b.fn
	for i := len(b.options) - 1; i >= 0; i-- {
		opt := b.options[i]
		next := call
		call = func(ctx context.Context) error {
			return opt(ctx, next)
		}
	}

	return call(ctx)
}

// mergeEvents collects Events from client plugins and call plugins.
func (b *CallBuilder) mergeEvents() []Events {
	var all []Events

	if b.client != nil {
		all = append(all, b.client.events...)
	}

	for _, p := range b.plugins {
		all = append(all, p.Events())
	}

	return all
}

// Do executes fn with the given options. Stateless shortcut — no client, no plugins.
//
// For plugin support, use [NewClient] to create a [Client].
func Do(ctx context.Context, fn func(context.Context) error, opts ...Option) error {
	call := fn

	for i := len(opts) - 1; i >= 0; i-- {
		opt := opts[i]
		next := call
		call = func(ctx context.Context) error {
			return opt(ctx, next)
		}
	}

	return call(ctx)
}

// --- context helpers ---

// eventsKey is the context key for plugin events.
type eventsKey struct{}

// withEvents attaches events to context. Called by [CallBuilder.Do].
func withEvents(ctx context.Context, events []Events) context.Context {
	return context.WithValue(ctx, eventsKey{}, events)
}

// EventsFromContext extracts plugin events from context.
// Returns nil when no plugins are active — options work without plugins,
// just without observability.
//
// For use by sub-packages that need to emit lifecycle events (e.g. retry):
//
//	events := resilience.EventsFromContext(ctx)
//	resilience.EmitBeforeWait(ctx, events, "node", attempt, wait)
func EventsFromContext(ctx context.Context) []Events {
	events, _ := ctx.Value(eventsKey{}).([]Events)

	return events
}

// --- event emitters ---

// EmitBeforeCall notifies all plugins that a call is about to start.
// For use by sub-packages that wrap calls (e.g. retry before each attempt).
func EmitBeforeCall(ctx context.Context, events []Events, attempt int) {
	for i := range events {
		if events[i].OnBeforeCall != nil {
			events[i].OnBeforeCall(ctx, attempt)
		}
	}
}

// EmitAfterCall notifies all plugins that a call has completed.
// For use by sub-packages that wrap calls (e.g. retry after each attempt).
func EmitAfterCall(ctx context.Context, events []Events, attempt int, err error, duration time.Duration) {
	for i := range events {
		if events[i].OnAfterCall != nil {
			events[i].OnAfterCall(ctx, attempt, err, duration)
		}
	}
}

// EmitBeforeWait notifies all plugins that an option is about to sleep.
// For use by sub-packages that implement backoff waits (e.g. retry).
func EmitBeforeWait(ctx context.Context, events []Events, option string, attempt int, wait time.Duration) {
	for i := range events {
		if events[i].OnBeforeWait != nil {
			events[i].OnBeforeWait(ctx, option, attempt, wait)
		}
	}
}
