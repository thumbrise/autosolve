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
	"testing"

	"github.com/thumbrise/autosolve/pkg/resilience"
)

func TestDo_Success(t *testing.T) {
	err := resilience.Do(context.Background(), func(context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestDo_Error(t *testing.T) {
	errFatal := errors.New("fatal")

	err := resilience.Do(context.Background(), func(context.Context) error {
		return errFatal
	})
	if !errors.Is(err, errFatal) {
		t.Fatalf("expected errFatal, got %v", err)
	}
}

func TestDo_MiddlewareOrder(t *testing.T) {
	// Middleware should wrap in order: first option is outermost.
	var order []string

	makeOpt := func(name string) resilience.Option {
		return resilience.NewOption(func(next resilience.Func) resilience.Func {
			return func(ctx context.Context) error {
				order = append(order, name+":before")
				err := next(ctx)

				order = append(order, name+":after")

				return err
			}
		})
	}

	err := resilience.Do(context.Background(), func(context.Context) error {
		order = append(order, "fn")

		return nil
	}, makeOpt("A"), makeOpt("B"), makeOpt("C"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"A:before", "B:before", "C:before", "fn", "C:after", "B:after", "A:after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}

	for i := range expected {
		if order[i] != expected[i] {
			t.Fatalf("at index %d: expected %q, got %q\nfull: %v", i, expected[i], order[i], order)
		}
	}
}

func TestCompose_MergesOptions(t *testing.T) {
	var order []string

	makeOpt := func(name string) resilience.Option {
		return resilience.NewOption(func(next resilience.Func) resilience.Func {
			return func(ctx context.Context) error {
				order = append(order, name)

				return next(ctx)
			}
		})
	}

	composed := resilience.Compose(makeOpt("A"), makeOpt("B"))

	err := resilience.Do(context.Background(), func(context.Context) error {
		order = append(order, "fn")

		return nil
	}, composed, makeOpt("C"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"A", "B", "C", "fn"}
	if len(order) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}

	for i := range expected {
		if order[i] != expected[i] {
			t.Fatalf("at index %d: expected %q, got %q", i, expected[i], order[i])
		}
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
