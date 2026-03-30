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

package longrun_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thumbrise/autosolve/pkg/longrun"
)

// --- constructor panics ---

func TestNewOneShotTask_PanicsOnNilWork(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	longrun.NewOneShotTask("test", nil, nil)
}

func TestNewIntervalTask_PanicsOnZeroInterval(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	longrun.NewIntervalTask("test", 0, func(context.Context) error { return nil }, nil)
}

func TestNewOneShotTask_PanicsOnZeroBackoffInitial(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for zero backoff Initial")
		}
	}()

	longrun.NewOneShotTask("test", func(context.Context) error { return nil }, []longrun.TransientRule{
		{Err: errSentinel, MaxRetries: 3, Backoff: longrun.BackoffConfig{
			Initial: 0, Max: time.Second, Multiplier: 2.0,
		}},
	})
}

func TestNewOneShotTask_PanicsOnZeroBackoffMultiplier(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for zero backoff Multiplier")
		}
	}()

	longrun.NewOneShotTask("test", func(context.Context) error { return nil }, []longrun.TransientRule{
		{Err: errSentinel, MaxRetries: 3, Backoff: longrun.BackoffConfig{
			Initial: time.Second, Max: 10 * time.Second, Multiplier: 0,
		}},
	})
}

// --- one-shot ---

func TestOneShotTask_Success(t *testing.T) {
	var called int32

	task := longrun.NewOneShotTask("test", func(context.Context) error {
		atomic.AddInt32(&called, 1)

		return nil
	}, nil)

	err := task.Wait(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if atomic.LoadInt32(&called) != 1 {
		t.Fatalf("expected 1 call, got %d", atomic.LoadInt32(&called))
	}
}

func TestOneShotTask_PermanentError_NoRules(t *testing.T) {
	task := longrun.NewOneShotTask("test", func(context.Context) error {
		return errors.New("fatal")
	}, nil)

	err := task.Wait(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOneShotTask_TransientRetry(t *testing.T) {
	var calls int32

	task := longrun.NewOneShotTask("test", func(context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return errSentinel
		}

		return nil
	}, []longrun.TransientRule{
		{Err: errSentinel, MaxRetries: 5, Backoff: longrun.BackoffConfig{
			Initial: 1 * time.Millisecond, Max: 10 * time.Millisecond, Multiplier: 2.0,
		}},
	})

	err := task.Wait(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", atomic.LoadInt32(&calls))
	}
}

func TestOneShotTask_MaxRetriesExhausted(t *testing.T) {
	task := longrun.NewOneShotTask("test", func(context.Context) error {
		return errSentinel
	}, []longrun.TransientRule{
		{Err: errSentinel, MaxRetries: 2, Backoff: longrun.BackoffConfig{
			Initial: 1 * time.Millisecond, Max: 10 * time.Millisecond, Multiplier: 2.0,
		}},
	})

	err := task.Wait(context.Background())
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
}

func TestOneShotTask_UnmatchedError_Permanent(t *testing.T) {
	errOther := errors.New("other")

	task := longrun.NewOneShotTask("test", func(context.Context) error {
		return errOther
	}, []longrun.TransientRule{
		{Err: errSentinel, MaxRetries: 5, Backoff: longrun.BackoffConfig{
			Initial: 1 * time.Millisecond, Max: 10 * time.Millisecond, Multiplier: 2.0,
		}},
	})

	err := task.Wait(context.Background())
	if !errors.Is(err, errOther) {
		t.Fatalf("expected errOther, got: %v", err)
	}
}

// --- context cancellation ---

func TestOneShotTask_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	task := longrun.NewOneShotTask("test", func(ctx context.Context) error {
		<-ctx.Done()

		return ctx.Err()
	}, nil)

	err := task.Wait(ctx)
	if err != nil {
		t.Fatalf("context cancellation should not be returned as error, got: %v", err)
	}
}

// --- interval ---

func TestIntervalTask_RunsMultipleTicks(t *testing.T) {
	var calls int32

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	task := longrun.NewIntervalTask("test", 10*time.Millisecond, func(context.Context) error {
		atomic.AddInt32(&calls, 1)

		return nil
	}, nil)

	err := task.Wait(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n := atomic.LoadInt32(&calls)
	if n < 2 {
		t.Fatalf("expected at least 2 ticks, got %d", n)
	}
}

func TestIntervalTask_ErrorKillsWithoutRules(t *testing.T) {
	var calls int32

	task := longrun.NewIntervalTask("test", 10*time.Millisecond, func(context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n == 2 {
			return errors.New("fatal")
		}

		return nil
	}, nil)

	err := task.Wait(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", atomic.LoadInt32(&calls))
	}
}

func TestIntervalTask_TransientRetryThenRecover(t *testing.T) {
	var calls int32

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	task := longrun.NewIntervalTask("test", 10*time.Millisecond, func(context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n == 2 {
			return errSentinel
		}

		return nil
	}, []longrun.TransientRule{
		{Err: errSentinel, MaxRetries: 3, Backoff: longrun.BackoffConfig{
			Initial: 1 * time.Millisecond, Max: 5 * time.Millisecond, Multiplier: 2.0,
		}},
	})

	err := task.Wait(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n := atomic.LoadInt32(&calls)
	if n < 3 {
		t.Fatalf("expected at least 3 calls (1 ok, 1 fail, 1+ ok), got %d", n)
	}
}

// --- delay ---

func TestOneShotTask_WithDelay(t *testing.T) {
	start := time.Now()

	task := longrun.NewOneShotTask("test", func(context.Context) error {
		return nil
	}, nil, longrun.WithDelay(50*time.Millisecond))

	err := task.Wait(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if elapsed < 40*time.Millisecond {
		t.Fatalf("expected delay of ~50ms, got %v", elapsed)
	}
}

func TestOneShotTask_WithDelay_NotReappliedOnRetry(t *testing.T) {
	delay := 50 * time.Millisecond

	var calls int32

	task := longrun.NewOneShotTask("test", func(context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return errSentinel
		}

		return nil
	}, []longrun.TransientRule{
		{Err: errSentinel, MaxRetries: 5, Backoff: longrun.BackoffConfig{
			Initial: 1 * time.Millisecond, Max: 5 * time.Millisecond, Multiplier: 2.0,
		}},
	}, longrun.WithDelay(delay))

	start := time.Now()
	err := task.Wait(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", atomic.LoadInt32(&calls))
	}

	// Expected: ~50ms delay + ~3ms backoff. If delay re-applied on each retry
	// it would be ~150ms+. We allow up to 100ms to catch the bug.
	if elapsed > 100*time.Millisecond {
		t.Fatalf("delay was re-applied on retry: total elapsed %v, expected ~55ms", elapsed)
	}
}

func TestOneShotTask_WithDelay_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	task := longrun.NewOneShotTask("test", func(context.Context) error {
		t.Fatal("work should not be called")

		return nil
	}, nil, longrun.WithDelay(10*time.Second))

	err := task.Wait(ctx)
	if err != nil {
		t.Fatalf("context cancellation should not be returned as error, got: %v", err)
	}
}

// --- baseline: out-of-range category ---

func TestBaseline_OutOfRangeCategory_NoPanic(t *testing.T) {
	// Regression test for #129: ClassifierFunc returns an ErrorCategory
	// outside the known range (e.g. 99). Before the fix, this panics with
	// "index out of range [99] with length 3" in retryWithPolicy.
	//
	// Expected behavior: unknown category falls back to Default (degraded)
	// policy, task retries and completes successfully.
	var calls int32

	runner := longrun.NewRunner(longrun.RunnerOptions{
		Baseline: longrun.NewBaselineDegraded(
			longrun.Policy{Backoff: longrun.Backoff(1*time.Millisecond, 10*time.Millisecond)},
			longrun.Policy{Backoff: longrun.Backoff(1*time.Millisecond, 10*time.Millisecond)},
			longrun.Policy{Backoff: longrun.Backoff(1*time.Millisecond, 10*time.Millisecond)},
			func(err error) *longrun.ErrorClass {
				return &longrun.ErrorClass{Category: longrun.ErrorCategory(99)}
			},
		),
	})

	runner.Add(longrun.NewOneShotTask("test", func(context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			return errors.New("classified as category 99")
		}

		return nil
	}, nil))

	err := runner.Wait(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Fatalf("expected 2 calls (1 fail + 1 success), got %d", n)
	}
}

// --- timeout ---

func TestOneShotTask_WithTimeout(t *testing.T) {
	task := longrun.NewOneShotTask("test", func(ctx context.Context) error {
		<-ctx.Done()

		return ctx.Err()
	}, nil, longrun.WithTimeout(20*time.Millisecond))

	err := task.Wait(context.Background())
	// Timeout causes context.DeadlineExceeded inside work, but the parent ctx is fine,
	// so it's returned as a permanent error (no rules).
	if err == nil {
		t.Fatal("expected error from timeout")
	}
}
