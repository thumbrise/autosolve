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

// Package resilience provides a composable single-call resilience primitive.
//
// The entry point is [Do] — execute a function with resilience patterns
// (retry, timeout, circuit breaker, etc.) applied as [Option] middleware.
//
// Each Option wraps the call with behavior. Options compose top-to-bottom
// as a middleware pipeline:
//
//	err := resilience.Do(ctx, func(ctx context.Context) error {
//	    return httpClient.Do(ctx, req)
//	},
//	    timeout.After(5*time.Second),
//	    retry.On(ErrTimeout, 3, backoff.Exponential(1*time.Second, 30*time.Second)),
//	)
package resilience

import "context"

// Func is the function to execute with resilience.
type Func func(ctx context.Context) error

// Middleware wraps a Func with additional behavior.
// It receives the next Func in the chain and returns a wrapped Func.
type Middleware func(next Func) Func

// Option configures a resilience pipeline. Each Option contributes
// a [Middleware] that wraps the call.
//
// Create via [NewOption]. Sub-packages use this to define their own options.
type Option struct {
	mw Middleware
}

// NewOption creates an Option from a Middleware function.
// This is the only way to construct an Option — sub-packages (retry, timeout,
// circuit, etc.) use it to build their options.
func NewOption(mw Middleware) Option {
	return Option{mw: mw}
}

// Do executes fn with the given resilience options applied as a middleware
// pipeline. Options are applied in order: the first option is the outermost
// wrapper, the last option is closest to fn.
//
// Returns nil on success, or the error from fn (or a middleware) on failure.
// Context cancellation is respected — if ctx is cancelled, Do returns
// ctx.Err() at the earliest opportunity.
func Do(ctx context.Context, fn Func, opts ...Option) error {
	// Build the middleware chain: last option wraps fn first,
	// first option is the outermost layer.
	wrapped := fn
	for i := len(opts) - 1; i >= 0; i-- {
		wrapped = opts[i].mw(wrapped)
	}

	return wrapped(ctx)
}

// Compose merges multiple Options into a single Option.
// The resulting Option applies all inner options in order.
// Useful for building presets.
//
//	preset := resilience.Compose(
//	    retry.On(ErrTimeout, 3, backoff.Exponential(1*time.Second, 30*time.Second)),
//	    retry.On(ErrDNS, 2, backoff.Constant(2*time.Second)),
//	)
func Compose(opts ...Option) Option {
	return NewOption(func(next Func) Func {
		wrapped := next
		for i := len(opts) - 1; i >= 0; i-- {
			wrapped = opts[i].mw(wrapped)
		}

		return wrapped
	})
}
