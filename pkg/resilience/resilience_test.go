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

package resilience_test

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

// --- Do (stateless shortcut) ---

func TestDo_Success(t *testing.T) {
	err := resilience.Do(context.Background(), func(context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestDo_Error_NoOptions(t *testing.T) {
	errFatal := errors.New("fatal")

	err := resilience.Do(context.Background(), func(context.Context) error {
		return errFatal
	})
	if !errors.Is(err, errFatal) {
		t.Fatalf("expected errFatal, got %v", err)
	}
}

func TestDo_NoOptions(t *testing.T) {
	called := false

	err := resilience.Do(context.Background(), func(context.Context) error {
		called = true

		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("fn was not called")
	}
}

func TestDo_RetryOption(t *testing.T) {
	errA := errors.New("error A")

	var calls int32

	err := resilience.Do(context.Background(), func(context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return errA
		}

		return nil
	},
		retry.On(errA, 5, backoff.Constant(1*time.Millisecond)),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n := atomic.LoadInt32(&calls); n != 3 {
		t.Fatalf("expected 3 calls, got %d", n)
	}
}

func TestDo_OptionOrder_OutermostFirst(t *testing.T) {
	var order []string

	makeOption := func(name string) resilience.Option {
		return func(ctx context.Context, call func(context.Context) error) error {
			order = append(order, name+":before")
			err := call(ctx)

			order = append(order, name+":after")

			return err
		}
	}

	err := resilience.Do(context.Background(), func(context.Context) error {
		order = append(order, "fn")

		return nil
	},
		makeOption("A"),
		makeOption("B"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"A:before", "B:before", "fn", "B:after", "A:after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}

	for i := range expected {
		if order[i] != expected[i] {
			t.Fatalf("at index %d: expected %q, got %q\nfull: %v", i, expected[i], order[i], order)
		}
	}
}

// --- Client + CallBuilder ---

// testPlugin is a minimal Plugin for testing.
type testPlugin struct {
	events resilience.Events
}

func (p *testPlugin) Name() string              { return "test" }
func (p *testPlugin) Events() resilience.Events { return p.events }

func TestClient_Call_WithPlugin(t *testing.T) {
	var beforeCalls, afterCalls int32

	client := resilience.NewClient(&testPlugin{
		events: resilience.Events{
			OnBeforeCall: func(_ context.Context, _ int) {
				atomic.AddInt32(&beforeCalls, 1)
			},
			OnAfterCall: func(_ context.Context, _ int, _ error, _ time.Duration) {
				atomic.AddInt32(&afterCalls, 1)
			},
		},
	})

	errTransient := errors.New("transient")

	var calls int32

	err := client.Call(func(context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return errTransient
		}

		return nil
	}).
		With(retry.On(errTransient, 5, backoff.Constant(1*time.Millisecond))).
		Do(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n := atomic.LoadInt32(&calls); n != 3 {
		t.Fatalf("expected 3 calls, got %d", n)
	}

	// Events should fire for each call (including retries).
	if n := atomic.LoadInt32(&beforeCalls); n != 3 {
		t.Fatalf("expected 3 OnBeforeCall events, got %d", n)
	}

	if n := atomic.LoadInt32(&afterCalls); n != 3 {
		t.Fatalf("expected 3 OnAfterCall events, got %d", n)
	}
}

func TestClient_Call_FreshPerCall(t *testing.T) {
	client := resilience.NewClient()

	errFail := errors.New("fail")

	// First call — retry exhausts budget.
	err := client.Call(func(context.Context) error {
		return errFail
	}).
		With(retry.On(errFail, 2, backoff.Constant(1*time.Millisecond))).
		Do(context.Background())

	if !errors.Is(err, errFail) {
		t.Fatalf("first call: expected errFail, got %v", err)
	}

	// Second call — fresh budget, not affected by first call.
	var calls int32

	err = client.Call(func(context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 2 {
			return errFail
		}

		return nil
	}).
		With(retry.On(errFail, 3, backoff.Constant(1*time.Millisecond))).
		Do(context.Background())
	if err != nil {
		t.Fatalf("second call: unexpected error: %v", err)
	}

	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Fatalf("expected 2 calls, got %d", n)
	}
}

func TestClient_Call_NoOptions(t *testing.T) {
	client := resilience.NewClient()

	called := false

	err := client.Call(func(context.Context) error {
		called = true

		return nil
	}).Do(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("fn was not called")
	}
}

// --- Context cancellation ---

func TestDo_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := resilience.Do(ctx, func(context.Context) error {
		t.Fatal("fn should not be called")

		return nil
	},
		retry.On(errors.New("any"), 5, backoff.Constant(1*time.Second)),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
