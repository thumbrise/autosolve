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
	"testing"

	"github.com/thumbrise/autosolve/pkg/longrun"
)

func TestRuleTracker_Limited_ExhaustesBudget(t *testing.T) {
	rt := longrun.NewRuleTracker(2)

	attempt, ok := rt.OnFailure()
	if !ok || attempt != 0 {
		t.Fatalf("1st: expected (0, true), got (%d, %v)", attempt, ok)
	}

	attempt, ok = rt.OnFailure()
	if !ok || attempt != 1 {
		t.Fatalf("2nd: expected (1, true), got (%d, %v)", attempt, ok)
	}

	_, ok = rt.OnFailure()
	if ok {
		t.Fatal("3rd: expected budget exhausted")
	}
}

func TestRuleTracker_ZeroValue_DefaultMaxRetries(t *testing.T) {
	rt := longrun.NewRuleTracker(0)

	if rt.Max() != longrun.DefaultMaxRetries {
		t.Fatalf("expected max=%d, got %d", longrun.DefaultMaxRetries, rt.Max())
	}

	for range longrun.DefaultMaxRetries {
		_, ok := rt.OnFailure()
		if !ok {
			t.Fatal("expected ok within default budget")
		}
	}

	_, ok := rt.OnFailure()
	if ok {
		t.Fatal("expected budget exhausted after default retries")
	}
}

func TestRuleTracker_Unlimited(t *testing.T) {
	rt := longrun.NewRuleTracker(longrun.UnlimitedRetries)

	for i := range 100 {
		attempt, ok := rt.OnFailure()
		if !ok {
			t.Fatalf("attempt %d: expected ok", i)
		}

		if attempt != i {
			t.Fatalf("expected attempt=%d, got %d", i, attempt)
		}
	}
}

func TestRuleTracker_Reset(t *testing.T) {
	rt := longrun.NewRuleTracker(2)

	rt.OnFailure()
	rt.OnFailure()

	// Budget exhausted.
	_, ok := rt.OnFailure()
	if ok {
		t.Fatal("expected budget exhausted before reset")
	}

	rt.Reset()

	if rt.Attempt() != 0 {
		t.Fatalf("expected attempt=0 after reset, got %d", rt.Attempt())
	}

	// Budget restored.
	attempt, ok := rt.OnFailure()
	if !ok || attempt != 0 {
		t.Fatalf("after reset: expected (0, true), got (%d, %v)", attempt, ok)
	}
}
