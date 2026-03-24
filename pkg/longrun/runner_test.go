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
	"sync"
	"testing"
	"time"

	"github.com/thumbrise/autosolve/pkg/longrun"
)

func TestRunner_AllTasksSucceed(t *testing.T) {
	runner := longrun.NewRunner(longrun.RunnerOptions{})

	runner.Add(longrun.NewOneShotTask("a", func(context.Context) error { return nil }, nil))
	runner.Add(longrun.NewOneShotTask("b", func(context.Context) error { return nil }, nil))

	err := runner.Wait(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunner_OneTaskFails_CancelsOthers(t *testing.T) {
	runner := longrun.NewRunner(longrun.RunnerOptions{})

	errFatal := errors.New("fatal")

	runner.Add(longrun.NewOneShotTask("fail", func(context.Context) error {
		return errFatal
	}, nil))

	runner.Add(longrun.NewIntervalTask("long", 10*time.Millisecond, func(ctx context.Context) error {
		<-ctx.Done()

		return nil
	}, nil))

	err := runner.Wait(context.Background())
	if !errors.Is(err, errFatal) {
		t.Fatalf("expected errFatal, got: %v", err)
	}
}

func TestRunner_ShutdownHooks_LIFO(t *testing.T) {
	runner := longrun.NewRunner(longrun.RunnerOptions{})

	var mu sync.Mutex

	var order []string

	runner.Add(longrun.NewOneShotTask("a", func(context.Context) error { return nil }, nil,
		longrun.WithShutdown(func(context.Context) error {
			mu.Lock()

			order = append(order, "a")

			mu.Unlock()

			return nil
		}),
	))

	runner.Add(longrun.NewOneShotTask("b", func(context.Context) error { return nil }, nil,
		longrun.WithShutdown(func(context.Context) error {
			mu.Lock()

			order = append(order, "b")

			mu.Unlock()

			return nil
		}),
	))

	runner.Add(longrun.NewOneShotTask("c", func(context.Context) error { return nil }, nil,
		longrun.WithShutdown(func(context.Context) error {
			mu.Lock()

			order = append(order, "c")

			mu.Unlock()

			return nil
		}),
	))

	err := runner.Wait(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(order) != 3 {
		t.Fatalf("expected 3 shutdown hooks, got %d", len(order))
	}

	// LIFO: c, b, a
	if order[0] != "c" || order[1] != "b" || order[2] != "a" {
		t.Fatalf("expected LIFO order [c b a], got %v", order)
	}
}

func TestRunner_ContextCancellation(t *testing.T) {
	runner := longrun.NewRunner(longrun.RunnerOptions{})

	ctx, cancel := context.WithCancel(context.Background())

	runner.Add(longrun.NewIntervalTask("ticker", 10*time.Millisecond, func(context.Context) error {
		return nil
	}, nil))

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := runner.Wait(ctx)
	if err != nil {
		t.Fatalf("context cancellation should not return error, got: %v", err)
	}
}

func TestRunner_ShutdownHookError_DoesNotBlock(t *testing.T) {
	runner := longrun.NewRunner(longrun.RunnerOptions{})

	runner.Add(longrun.NewOneShotTask("a", func(context.Context) error { return nil }, nil,
		longrun.WithShutdown(func(context.Context) error {
			return errors.New("shutdown failed")
		}),
	))

	runner.Add(longrun.NewOneShotTask("b", func(context.Context) error { return nil }, nil,
		longrun.WithShutdown(func(context.Context) error {
			return nil
		}),
	))

	err := runner.Wait(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
