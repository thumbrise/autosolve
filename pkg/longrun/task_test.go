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
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thumbrise/autosolve/pkg/longrun"
)

var (
	errTransient = errors.New("transient")
	errPermanent = errors.New("permanent")
)

func fastBackoff() longrun.BackoffConfig {
	return longrun.BackoffConfig{
		Initial:    1 * time.Millisecond,
		Max:        1 * time.Millisecond,
		Multiplier: 1.0,
	}
}

// --- One-shot mode ---

func TestTask_OneShot_Success(t *testing.T) {
	var calls atomic.Int32

	task := longrun.NewTask("test", func(ctx context.Context) error {
		calls.Add(1)

		return nil
	}, longrun.TaskOptions{})

	err := task.Wait(context.Background())
	if err != nil {
		t.Fatalf("Wait() = %v, want nil", err)
	}

	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", calls.Load())
	}
}

func TestTask_OneShot_Error_RestartNever(t *testing.T) {
	task := longrun.NewTask("test", func(ctx context.Context) error {
		return errPermanent
	}, longrun.TaskOptions{})

	err := task.Wait(context.Background())
	if !errors.Is(err, errPermanent) {
		t.Fatalf("Wait() = %v, want %v", err, errPermanent)
	}
}

func TestTask_OneShot_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	task := longrun.NewTask("test", func(ctx context.Context) error {
		return ctx.Err()
	}, longrun.TaskOptions{})

	err := task.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait() = %v, want nil (cancelled context is not a task error)", err)
	}
}

// --- Restart policies ---

func TestTask_OnFailure_RetriesTransientErrors(t *testing.T) {
	var calls atomic.Int32

	task := longrun.NewTask("test", func(ctx context.Context) error {
		n := calls.Add(1)
		if n < 3 {
			return errTransient
		}

		return nil
	}, longrun.TaskOptions{
		Restart:         longrun.OnFailure,
		Backoff:         fastBackoff(),
		TransientErrors: []error{errTransient},
	})

	err := task.Wait(context.Background())
	if err != nil {
		t.Fatalf("Wait() = %v, want nil", err)
	}

	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3", calls.Load())
	}
}

func TestTask_OnFailure_PermanentErrorStops(t *testing.T) {
	var calls atomic.Int32

	task := longrun.NewTask("test", func(ctx context.Context) error {
		calls.Add(1)

		return errPermanent
	}, longrun.TaskOptions{
		Restart:         longrun.OnFailure,
		Backoff:         fastBackoff(),
		TransientErrors: []error{errTransient},
	})

	err := task.Wait(context.Background())
	if !errors.Is(err, errPermanent) {
		t.Fatalf("Wait() = %v, want %v", err, errPermanent)
	}

	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1 (permanent error should stop immediately)", calls.Load())
	}
}

func TestTask_OnFailure_EmptyTransientErrors_AllPermanent(t *testing.T) {
	var calls atomic.Int32

	task := longrun.NewTask("test", func(ctx context.Context) error {
		calls.Add(1)

		return errTransient // will be treated as permanent
	}, longrun.TaskOptions{
		Restart:         longrun.OnFailure,
		Backoff:         fastBackoff(),
		TransientErrors: nil, // empty = all permanent
	})

	err := task.Wait(context.Background())
	if !errors.Is(err, errTransient) {
		t.Fatalf("Wait() = %v, want %v", err, errTransient)
	}

	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", calls.Load())
	}
}

func TestTask_OnFailure_WrappedTransientError_Retries(t *testing.T) {
	var calls atomic.Int32

	task := longrun.NewTask("test", func(ctx context.Context) error {
		n := calls.Add(1)
		if n < 3 {
			return fmt.Errorf("wrapped: %w", errTransient)
		}

		return nil
	}, longrun.TaskOptions{
		Restart:         longrun.OnFailure,
		Backoff:         fastBackoff(),
		TransientErrors: []error{errTransient},
	})

	err := task.Wait(context.Background())
	if err != nil {
		t.Fatalf("Wait() = %v, want nil (wrapped transient should be retried)", err)
	}

	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3", calls.Load())
	}
}

// --- MaxRetries ---

func TestTask_MaxRetries_StopsAfterLimit(t *testing.T) {
	var calls atomic.Int32

	b := fastBackoff()
	b.MaxRetries = 3

	task := longrun.NewTask("test", func(ctx context.Context) error {
		calls.Add(1)

		return errTransient
	}, longrun.TaskOptions{
		Restart:         longrun.OnFailure,
		Backoff:         b,
		TransientErrors: []error{errTransient},
	})

	err := task.Wait(context.Background())
	if !errors.Is(err, errTransient) {
		t.Fatalf("Wait() = %v, want %v", err, errTransient)
	}

	// 1 initial + 3 retries = 4
	if calls.Load() != 4 {
		t.Fatalf("calls = %d, want 4 (1 initial + 3 retries)", calls.Load())
	}
}

func TestTask_MaxRetries_ResetsAfterSuccessfulRunLoop(t *testing.T) {
	// Scenario: interval task with OnFailure + MaxRetries=2.
	// Transient errors are separated by successful runLoop cycles.
	// The attempt counter must reset after each successful cycle,
	// so intermittent failures don't accumulate toward MaxRetries.
	//
	// Timeline:
	//   runLoop#1: call 1 ok, call 2 transient → retry (attempt 0→1)
	//   runLoop#2: call 3 ok, call 4 transient → retry (attempt must be 0→1 again, NOT 1→2)
	//   runLoop#3: call 5 ok, call 6 transient → retry (attempt must be 0→1 again)
	//   runLoop#4: call 7 ok → cancel
	//
	// Without the reset, attempt would reach MaxRetries=2 at the second
	// transient error and the task would stop permanently.
	var calls atomic.Int32

	ctx, cancel := context.WithCancel(context.Background())

	b := fastBackoff()
	b.MaxRetries = 2

	task := longrun.NewTask("test", func(ctx context.Context) error {
		n := calls.Add(1)

		// Even calls produce transient errors, odd calls succeed.
		// Each runLoop: immediate call (odd, ok) → tick call (even, transient).
		switch {
		case n >= 7:
			cancel()

			return nil
		case n%2 == 0:
			return errTransient
		default:
			return nil
		}
	}, longrun.TaskOptions{
		Interval:        1 * time.Millisecond,
		Restart:         longrun.OnFailure,
		Backoff:         b,
		TransientErrors: []error{errTransient},
	})

	err := task.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait() = %v, want nil (attempt counter should reset after successful runLoop)", err)
	}

	if calls.Load() < 7 {
		t.Fatalf("calls = %d, want >= 7", calls.Load())
	}
}

// --- Interval mode ---

func TestTask_Interval_RunsImmediatelyThenOnTick(t *testing.T) {
	var calls atomic.Int32

	ctx, cancel := context.WithCancel(context.Background())

	task := longrun.NewTask("test", func(ctx context.Context) error {
		n := calls.Add(1)
		if n >= 3 {
			cancel()
		}

		return nil
	}, longrun.TaskOptions{
		Interval: 1 * time.Millisecond,
	})

	err := task.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait() = %v, want nil", err)
	}

	if calls.Load() < 3 {
		t.Fatalf("calls = %d, want >= 3", calls.Load())
	}
}

func TestTask_Interval_SkipInitialRun(t *testing.T) {
	var calls atomic.Int32

	firstCallAt := make(chan time.Time, 1)

	ctx, cancel := context.WithCancel(context.Background())

	task := longrun.NewTask("test", func(ctx context.Context) error {
		n := calls.Add(1)
		if n == 1 {
			firstCallAt <- time.Now()
		}

		if n >= 2 {
			cancel()
		}

		return nil
	}, longrun.TaskOptions{
		Interval:       10 * time.Millisecond,
		SkipInitialRun: true,
	})

	start := time.Now()

	err := task.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait() = %v, want nil", err)
	}

	first := <-firstCallAt
	if first.Sub(start) < 5*time.Millisecond {
		t.Fatalf("first call was too soon (%v), expected delay from SkipInitialRun", first.Sub(start))
	}
}

func TestTask_Interval_ErrorBreaksLoop_ThenRetries(t *testing.T) {
	var calls atomic.Int32

	task := longrun.NewTask("test", func(ctx context.Context) error {
		n := calls.Add(1)
		if n == 2 {
			return errTransient // second tick errors
		}

		if n >= 4 {
			return errPermanent // stop
		}

		return nil
	}, longrun.TaskOptions{
		Interval:        1 * time.Millisecond,
		Restart:         longrun.OnFailure,
		Backoff:         fastBackoff(),
		TransientErrors: []error{errTransient},
	})

	err := task.Wait(context.Background())
	if !errors.Is(err, errPermanent) {
		t.Fatalf("Wait() = %v, want %v", err, errPermanent)
	}

	// Call 1: ok, Call 2: transient → retry → Call 3 (new loop, immediate): ok, Call 4: permanent
	if calls.Load() < 4 {
		t.Fatalf("calls = %d, want >= 4", calls.Load())
	}
}

// --- Timeout ---

func TestTask_Timeout_CancelsSlowWork(t *testing.T) {
	task := longrun.NewTask("test", func(ctx context.Context) error {
		<-ctx.Done()

		return ctx.Err()
	}, longrun.TaskOptions{
		Timeout: 5 * time.Millisecond,
	})

	start := time.Now()

	err := task.Wait(context.Background())

	elapsed := time.Since(start)

	// context.DeadlineExceeded is permanent (TransientErrors is empty)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait() = %v, want context.DeadlineExceeded", err)
	}

	if elapsed > 1*time.Second {
		t.Fatalf("took %v, timeout should have kicked in at 5ms", elapsed)
	}
}

// --- NewTask panics ---

func TestNewTask_PanicsOnNilWork(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewTask(nil work) should panic")
		}
	}()

	longrun.NewTask("test", nil, longrun.TaskOptions{})
}
