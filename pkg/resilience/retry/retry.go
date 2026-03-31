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

// Package retry provides a retry [resilience.Option] for [resilience.Do].
//
// Each [On] call creates an independent retry rule: its own error matcher,
// budget, backoff curve, and attempt counter. Multiple On options compose
// naturally — the first matching rule handles the error.
//
//	resilience.Do(ctx, fn,
//	    retry.On(ErrTimeout, 3, backoff.Exponential(1*time.Second, 30*time.Second)),
//	    retry.On(ErrRateLimit, 5, backoff.Constant(10*time.Second)),
//	)
package retry

import (
	"context"
	"errors"
	"log/slog"
	"reflect"

	"github.com/thumbrise/autosolve/pkg/resilience"
	"github.com/thumbrise/autosolve/pkg/resilience/backoff"
)

// Unlimited disables the retry limit — retries forever
// (until success or context cancellation).
const Unlimited = -1

// On creates a resilience Option that retries when the call returns an error
// matching errVal.
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
func On(errVal error, maxRetries int, bo backoff.Func) resilience.Option {
	if errVal == nil {
		panic("retry.On: errVal must not be nil")
	}

	if bo == nil {
		panic("retry.On: backoff must not be nil")
	}

	return newRetryOption(newMatcher(errVal), maxRetries, bo, "retry")
}

// OnFunc creates a resilience Option that retries when the provided
// classifier function returns true for the error.
//
// This is the escape hatch for errors that can't be matched by errors.Is/As
// (e.g. HTTP status codes, custom predicates).
//
// Panics if classify or bo is nil.
func OnFunc(classify func(error) bool, maxRetries int, bo backoff.Func, name string) resilience.Option {
	if classify == nil {
		panic("retry.OnFunc: classify must not be nil")
	}

	if bo == nil {
		panic("retry.OnFunc: backoff must not be nil")
	}

	if name == "" {
		name = "custom"
	}

	return newRetryOption(classify, maxRetries, bo, "retry("+name+")")
}

// newRetryOption builds the common retry middleware.
// match decides whether the error is retryable.
// logPrefix is used in log messages (e.g. "retry" or "retry(custom)").
func newRetryOption(match func(error) bool, maxRetries int, bo backoff.Func, logPrefix string) resilience.Option {
	logMsg := logPrefix + ": transient error, retrying"

	return resilience.NewOption(func(next resilience.Func) resilience.Func {
		var attempt int

		logger := slog.Default()

		return func(ctx context.Context) error {
			for {
				err := next(ctx)
				if err == nil {
					attempt = 0

					return nil
				}

				if ctx.Err() != nil {
					return ctx.Err()
				}

				if !match(err) {
					return err
				}

				if maxRetries != Unlimited && attempt >= maxRetries {
					return err
				}

				wait := bo(attempt)

				logger.InfoContext(ctx, logMsg,
					slog.Int("attempt", attempt+1),
					slog.Any("error", err),
					slog.Any("backoff", wait),
				)

				attempt++

				resilience.SleepCtx(ctx, wait)

				if ctx.Err() != nil {
					return ctx.Err()
				}
			}
		}
	})
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
