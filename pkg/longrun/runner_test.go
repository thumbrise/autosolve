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

func TestRunner_AllTasksComplete(t *testing.T) {
	var count atomic.Int32

	runner := longrun.NewRunner(longrun.RunnerOptions{})

	for range 3 {
		runner.Add(longrun.NewTask("t", func(ctx context.Context) error {
			count.Add(1)

			return nil
		}, longrun.TaskOptions{}))
	}

	err := runner.Wait(context.Background())
	if err != nil {
		t.Fatalf("Wait() = %v, want nil", err)
	}

	if count.Load() != 3 {
		t.Fatalf("count = %d, want 3", count.Load())
	}
}

func TestRunner_OneTaskError_CancelsOthers(t *testing.T) {
	runner := longrun.NewRunner(longrun.RunnerOptions{})

	// This task errors immediately.
	runner.Add(longrun.NewTask("fail", func(ctx context.Context) error {
		return errPermanent
	}, longrun.TaskOptions{}))

	// This task blocks until cancelled.
	var cancelled atomic.Bool

	runner.Add(longrun.NewTask("block", func(ctx context.Context) error {
		<-ctx.Done()
		cancelled.Store(true)

		return nil
	}, longrun.TaskOptions{}))

	err := runner.Wait(context.Background())
	if !errors.Is(err, errPermanent) {
		t.Fatalf("Wait() = %v, want %v", err, errPermanent)
	}

	if !cancelled.Load() {
		t.Fatal("blocking task was not cancelled")
	}
}

func TestRunner_ShutdownCalled(t *testing.T) {
	var shutdownCalled atomic.Bool

	runner := longrun.NewRunner(longrun.RunnerOptions{})

	task := longrun.NewTask("t", func(ctx context.Context) error {
		return nil
	}, longrun.TaskOptions{})
	task.Shutdown = func(ctx context.Context) error {
		shutdownCalled.Store(true)

		return nil
	}

	runner.Add(task)

	err := runner.Wait(context.Background())
	if err != nil {
		t.Fatalf("Wait() = %v, want nil", err)
	}

	if !shutdownCalled.Load() {
		t.Fatal("Shutdown was not called")
	}
}

func TestRunner_ContextCancelled(t *testing.T) {
	runner := longrun.NewRunner(longrun.RunnerOptions{})

	runner.Add(longrun.NewTask("block", func(ctx context.Context) error {
		<-ctx.Done()

		return nil
	}, longrun.TaskOptions{}))

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	err := runner.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait() = %v, want nil (context cancellation is not an error)", err)
	}
}
