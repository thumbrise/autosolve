// Copyright 2026 thumbrise
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package longrun_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/thumbrise/autosolve/pkg/longrun"
)

// ---------------------------------------------------------------------------
// RestartStrategy
// ---------------------------------------------------------------------------
func TestNeverRestart(t *testing.T) {
	s := longrun.NeverRestart{}
	if s.ShouldRestart(nil) {
		t.Error("ShouldRestart(nil) = true, want false")
	}

	if s.ShouldRestart(errors.New("err")) {
		t.Error("ShouldRestart(err) = true, want false")
	}
}

func TestRestartOnFailure(t *testing.T) {
	s := longrun.RestartOnFailure{}
	if s.ShouldRestart(nil) {
		t.Error("ShouldRestart(nil) = true, want false")
	}

	if !s.ShouldRestart(errors.New("err")) {
		t.Error("ShouldRestart(err) = false, want true")
	}
}

// ---------------------------------------------------------------------------
// ErrorClassifier
// ---------------------------------------------------------------------------
func TestAllPermanent(t *testing.T) {
	c := longrun.AllPermanent{}
	if c.IsTransient(errors.New("any")) {
		t.Error("IsTransient() = true, want false")
	}
}

func TestWhitelistClassifier(t *testing.T) {
	errA := errors.New("a")
	errB := errors.New("b")
	errC := errors.New("c")
	c := longrun.NewWhitelistClassifier(errA, errB)

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"exact match A", errA, true},
		{"exact match B", errB, true},
		{"not listed", errC, false},
		{"wrapped match", fmt.Errorf("wrap: %w", errA), true},
		{"wrapped not listed", fmt.Errorf("wrap: %w", errC), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.IsTransient(tt.err); got != tt.want {
				t.Errorf("IsTransient(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AttemptTracker
// ---------------------------------------------------------------------------
func TestUnlimitedAttempts(t *testing.T) {
	a := longrun.NewUnlimitedAttempts()
	for i := range 100 {
		attempt, ok := a.OnFailure()
		if !ok {
			t.Fatalf("OnFailure() = false at iteration %d", i)
		}

		if attempt != i {
			t.Fatalf("attempt = %d, want %d", attempt, i)
		}
	}

	a.Reset()

	attempt, ok := a.OnFailure()
	if !ok {
		t.Fatal("OnFailure() after Reset() = false, want true")
	}

	if attempt != 0 {
		t.Fatalf("attempt after Reset() = %d, want 0", attempt)
	}
}

func TestLimitedAttempts(t *testing.T) {
	a := longrun.NewLimitedAttempts(3)
	// 3 retries allowed: attempts 0, 1, 2.
	for i := range 3 {
		attempt, ok := a.OnFailure()
		if !ok {
			t.Fatalf("OnFailure() = false at call %d, want true", i+1)
		}

		if attempt != i {
			t.Fatalf("attempt = %d, want %d", attempt, i)
		}
	}
	// 4th call — budget exhausted.
	_, ok := a.OnFailure()
	if ok {
		t.Fatal("OnFailure() on 4th call = true, want false (budget exhausted)")
	}
	// Reset restores budget.
	a.Reset()

	attempt, ok := a.OnFailure()
	if !ok {
		t.Fatal("OnFailure() after Reset() = false, want true")
	}

	if attempt != 0 {
		t.Fatalf("attempt after Reset() = %d, want 0", attempt)
	}
}
