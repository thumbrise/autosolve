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

package retry_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thumbrise/autosolve/pkg/resilience"
	"github.com/thumbrise/autosolve/pkg/resilience/backoff"
	"github.com/thumbrise/autosolve/pkg/resilience/retry"
)

var errTransient = errors.New("transient")

func TestOn_Success_NoRetry(t *testing.T) {
	var calls int32

	err := resilience.Do(context.Background(), func(context.Context) error {
		atomic.AddInt32(&calls, 1)

		return nil
	}, retry.On(errTransient, 3, backoff.Constant(1*time.Millisecond)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("expected 1 call, got %d", n)
	}
}

func TestOn_RetryThenSuccess(t *testing.T) {
	var calls int32

	err := resilience.Do(context.Background(), func(context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return errTransient
		}

		return nil
	}, retry.On(errTransient, 5, backoff.Constant(1*time.Millisecond)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n := atomic.LoadInt32(&calls); n != 3 {
		t.Fatalf("expected 3 calls, got %d", n)
	}
}

func TestOn_BudgetExhausted(t *testing.T) {
	var calls int32

	err := resilience.Do(context.Background(), func(context.Context) error {
		atomic.AddInt32(&calls, 1)

		return errTransient
	}, retry.On(errTransient, 2, backoff.Constant(1*time.Millisecond)))

	if !errors.Is(err, errTransient) {
		t.Fatalf("expected errTransient, got %v", err)
	}

	// 1 initial + 2 retries = 3 calls
	if n := atomic.LoadInt32(&calls); n != 3 {
		t.Fatalf("expected 3 calls (1 + 2 retries), got %d", n)
	}
}

func TestOn_UnmatchedError_NoRetry(t *testing.T) {
	errOther := errors.New("other")

	var calls int32

	err := resilience.Do(context.Background(), func(context.Context) error {
		atomic.AddInt32(&calls, 1)

		return errOther
	}, retry.On(errTransient, 5, backoff.Constant(1*time.Millisecond)))

	if !errors.Is(err, errOther) {
		t.Fatalf("expected errOther, got %v", err)
	}

	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("expected 1 call (no retry for unmatched error), got %d", n)
	}
}

func TestOn_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var calls int32

	err := resilience.Do(ctx, func(context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			cancel()

			return errTransient
		}

		return nil
	}, retry.On(errTransient, 10, backoff.Constant(1*time.Millisecond)))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestOn_Unlimited(t *testing.T) {
	var calls int32

	err := resilience.Do(context.Background(), func(context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 10 {
			return errTransient
		}

		return nil
	}, retry.On(errTransient, retry.Unlimited, backoff.Constant(1*time.Millisecond)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n := atomic.LoadInt32(&calls); n != 10 {
		t.Fatalf("expected 10 calls, got %d", n)
	}
}

func TestOn_MultipleRules_FirstMatchWins(t *testing.T) {
	errA := errors.New("error A")
	errB := errors.New("error B")

	var calls int32

	err := resilience.Do(context.Background(), func(context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			return errA
		}

		if n == 2 {
			return errB
		}

		return nil
	},
		retry.On(errA, 3, backoff.Constant(1*time.Millisecond)),
		retry.On(errB, 3, backoff.Constant(1*time.Millisecond)),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n := atomic.LoadInt32(&calls); n != 3 {
		t.Fatalf("expected 3 calls, got %d", n)
	}
}

func TestOn_PanicsOnNilErr(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	retry.On(nil, 3, backoff.Constant(1*time.Millisecond))
}

func TestOn_PanicsOnNilBackoff(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	retry.On(errTransient, 3, nil)
}

func TestOn_PanicsOnZeroMaxRetries(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for maxRetries=0")
		}
	}()

	retry.On(errTransient, 0, backoff.Constant(1*time.Millisecond))
}

func TestOn_PanicsOnInvalidNegativeMaxRetries(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for maxRetries=-5")
		}
	}()

	retry.On(errTransient, -5, backoff.Constant(1*time.Millisecond))
}

func TestOn_WrappedError(t *testing.T) {
	wrapped := errors.New("wrapped: transient")

	var calls int32

	err := resilience.Do(context.Background(), func(context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			return wrapped
		}

		return nil
	}, retry.On(wrapped, 3, backoff.Constant(1*time.Millisecond)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Fatalf("expected 2 calls, got %d", n)
	}
}

func TestOnFunc_CustomClassifier(t *testing.T) {
	errCode := errors.New("status 503")

	classify := func(err error) bool {
		return err.Error() == "status 503"
	}

	var calls int32

	err := resilience.Do(context.Background(), func(context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return errCode
		}

		return nil
	}, retry.OnFunc(classify, 5, backoff.Constant(1*time.Millisecond), "http503"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n := atomic.LoadInt32(&calls); n != 3 {
		t.Fatalf("expected 3 calls, got %d", n)
	}
}

func TestOn_WithWaitHint_OverridesBackoff(t *testing.T) {
	hint := 1 * time.Millisecond

	var calls int32

	err := resilience.Do(context.Background(), func(context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 2 {
			return errTransient
		}

		return nil
	}, retry.On(errTransient, 3, backoff.Constant(10*time.Second),
		retry.WithWaitHint(func(error) time.Duration {
			return hint
		}),
	))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// If backoff (10s) was used instead of hint (1ms), test would timeout.
	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Fatalf("expected 2 calls, got %d", n)
	}
}

func TestOn_WithWaitHint_ZeroFallsBackToBackoff(t *testing.T) {
	var calls int32

	err := resilience.Do(context.Background(), func(context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 2 {
			return errTransient
		}

		return nil
	}, retry.On(errTransient, 3, backoff.Constant(1*time.Millisecond),
		retry.WithWaitHint(func(error) time.Duration {
			return 0 // no hint — backoff should be used
		}),
	))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Fatalf("expected 2 calls, got %d", n)
	}
}

func TestOn_BackoffReceivesCorrectAttemptIndex(t *testing.T) {
	var attempts []int

	bo := func(attempt int) time.Duration {
		attempts = append(attempts, attempt)

		return 1 * time.Millisecond
	}

	resilience.Do(context.Background(), func(context.Context) error {
		return errTransient
	}, retry.On(errTransient, 3, bo))

	expected := []int{0, 1, 2}
	if len(attempts) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, attempts)
	}

	for i := range expected {
		if attempts[i] != expected[i] {
			t.Fatalf("at index %d: expected attempt %d, got %d", i, expected[i], attempts[i])
		}
	}
}
