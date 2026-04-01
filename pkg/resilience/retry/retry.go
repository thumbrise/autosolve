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

// Package retry provides retry [resilience.Option] implementations.
//
// Each [On] / [OnFunc] call returns an [resilience.Option] that wraps
// the call with retry logic: match error, compute backoff, sleep, retry.
// Each Option owns its state (attempt counter) — no shared mutable state.
//
//	resilience.Do(ctx, fn,
//	    retry.On(ErrTimeout, 3, backoff.Exponential(1*time.Second, 30*time.Second)),
//	    retry.On(ErrRateLimit, 5, backoff.Constant(10*time.Second)),
//	)
package retry

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/thumbrise/autosolve/pkg/resilience"
	"github.com/thumbrise/autosolve/pkg/resilience/backoff"
)

// Unlimited disables the retry limit — retries forever
// (until success or context cancellation).
const Unlimited = -1

// Opt configures a retry option. Use With* functions to create options.
type Opt func(*config)

// config holds retry parameters. Built once, used inside the Option closure.
type config struct {
	name       string
	match      func(error) bool
	maxRetries int
	bo         backoff.Func
	waitHint   func(error) time.Duration
}

// WithWaitHint extracts a server-suggested wait duration from the error.
// When the function returns > 0, it overrides the backoff calculation.
// When it returns 0, the normal backoff is used.
//
// Typical use: Retry-After header on HTTP 429.
//
//	retry.On(ErrRateLimit, 5, backoff.Exponential(5*time.Second, 5*time.Minute),
//	    retry.WithWaitHint(func(err error) time.Duration {
//	        var rl *github.RateLimitError
//	        if errors.As(err, &rl) {
//	            return rl.RetryAfter
//	        }
//	        return 0
//	    }),
//	)
func WithWaitHint(fn func(error) time.Duration) Opt {
	return func(c *config) {
		c.waitHint = fn
	}
}

// On creates a [resilience.Option] that retries when the call returns an error
// matching errVal via errors.Is / errors.As.
//
// errVal accepts two forms:
//   - error value (sentinel): matched via errors.Is
//   - *T where T implements error: matched via errors.As
//
// maxRetries semantics:
//
//	Unlimited (-1) → no limit.
//	>0 → exact retry count.
//
// Panics if errVal is nil, unsupported type, or bo is nil.
func On(errVal error, maxRetries int, bo backoff.Func, opts ...Opt) resilience.Option {
	if errVal == nil {
		panic("retry.On: errVal must not be nil")
	}

	if bo == nil {
		panic("retry.On: backoff must not be nil")
	}

	validateMaxRetries("retry.On", maxRetries)

	return newOption(newMatcher(errVal), maxRetries, bo, "retry", opts)
}

// OnFunc creates a [resilience.Option] that retries when the provided
// classifier function returns true for the error.
//
// This is the escape hatch for errors that can't be matched by errors.Is/As
// (e.g. HTTP status codes, custom predicates).
//
// Panics if classify or bo is nil.
func OnFunc(classify func(error) bool, maxRetries int, bo backoff.Func, name string, opts ...Opt) resilience.Option {
	if classify == nil {
		panic("retry.OnFunc: classify must not be nil")
	}

	if bo == nil {
		panic("retry.OnFunc: backoff must not be nil")
	}

	validateMaxRetries("retry.OnFunc", maxRetries)

	if name == "" {
		name = "custom"
	}

	return newOption(classify, maxRetries, bo, name, opts)
}

// newOption builds the retry Option. The returned function owns its own
// attempt counter — each call to On/OnFunc produces an independent Option.
func newOption(match func(error) bool, maxRetries int, bo backoff.Func, name string, opts []Opt) resilience.Option {
	cfg := &config{
		name:       name,
		match:      match,
		maxRetries: maxRetries,
		bo:         bo,
	}

	for _, o := range opts {
		o(cfg)
	}

	return func(ctx context.Context, call func(context.Context) error) error {
		return retryLoop(ctx, call, cfg)
	}
}

// retryLoop is the explicit retry loop. Each invocation has its own attempt counter.
func retryLoop(ctx context.Context, call func(context.Context) error, cfg *config) error {
	events := resilience.EventsFromContext(ctx)

	for attempt := 0; ; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := invokeCall(ctx, call, events, attempt)
		if err == nil {
			return nil
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if !cfg.match(err) {
			return err
		}

		if cfg.maxRetries != Unlimited && attempt >= cfg.maxRetries {
			return err
		}

		wait := computeWait(cfg, err, attempt)

		resilience.EmitBeforeWait(ctx, events, cfg.name, attempt, wait)
		resilience.SleepCtx(ctx, wait)
	}
}

// invokeCall calls fn and emits before/after events.
func invokeCall(ctx context.Context, call func(context.Context) error, events []resilience.Events, attempt int) error {
	resilience.EmitBeforeCall(ctx, events, attempt)

	start := time.Now()
	err := call(ctx)
	duration := time.Since(start)

	resilience.EmitAfterCall(ctx, events, attempt, err, duration)

	return err
}

// computeWait returns the wait duration: hint overrides backoff when > 0.
func computeWait(cfg *config, err error, attempt int) time.Duration {
	wait := cfg.bo(attempt)

	if cfg.waitHint != nil {
		if hint := cfg.waitHint(err); hint > 0 {
			wait = hint
		}
	}

	return wait
}

// validateMaxRetries panics on invalid maxRetries values.
// Valid: Unlimited (-1) or > 0. Zero and other negatives are programmer errors.
func validateMaxRetries(caller string, maxRetries int) {
	if maxRetries == 0 || maxRetries < Unlimited {
		panic(fmt.Sprintf("%s: maxRetries must be Unlimited (-1) or > 0, got %d", caller, maxRetries))
	}
}

// newMatcher compiles an error pattern into a match function.
//
// Two forms:
//   - error value (sentinel) → errors.Is
//   - *T where T implements error (typed nil pointer) → errors.As
//
// Panics on nil or unsupported type.
func newMatcher(errVal error) func(error) bool {
	// Case 1: *T where T implements error → errors.As.
	rv := reflect.ValueOf(errVal)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		errorIface := reflect.TypeOf((*error)(nil)).Elem()
		if rv.Type().Implements(errorIface) {
			targetType := rv.Type()

			return func(err error) bool {
				target := reflect.New(targetType)

				return errors.As(err, target.Interface())
			}
		}
	}

	// Case 2: error value (sentinel) → errors.Is.
	return func(err error) bool {
		return errors.Is(err, errVal)
	}
}
