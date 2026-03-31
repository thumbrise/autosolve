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

package backoff_test

import (
	"testing"
	"time"

	"github.com/thumbrise/autosolve/pkg/resilience/backoff"
)

func TestExponential_ClassicDoubling(t *testing.T) {
	bo := backoff.Exponential(1*time.Second, 30*time.Second)

	if d := bo(0); d != 1*time.Second {
		t.Fatalf("attempt 0: expected 1s, got %v", d)
	}

	if d := bo(1); d != 2*time.Second {
		t.Fatalf("attempt 1: expected 2s, got %v", d)
	}

	if d := bo(3); d != 8*time.Second {
		t.Fatalf("attempt 3: expected 8s, got %v", d)
	}
}

func TestExponential_CapsAtMax(t *testing.T) {
	bo := backoff.ExponentialWith(1*time.Second, 5*time.Second, 10.0)

	if d := bo(0); d != 1*time.Second {
		t.Fatalf("attempt 0: expected 1s, got %v", d)
	}

	// 1s * 10^1 = 10s → capped at 5s
	if d := bo(1); d != 5*time.Second {
		t.Fatalf("attempt 1: expected 5s (capped), got %v", d)
	}
}

func TestExponential_NoMaxNoCap(t *testing.T) {
	bo := backoff.ExponentialWith(1*time.Second, 0, 2.0)

	if d := bo(3); d != 8*time.Second {
		t.Fatalf("expected 8s, got %v", d)
	}
}

func TestConstant_AlwaysSameDuration(t *testing.T) {
	bo := backoff.Constant(5 * time.Second)

	for attempt := range 10 {
		if d := bo(attempt); d != 5*time.Second {
			t.Fatalf("attempt %d: expected 5s, got %v", attempt, d)
		}
	}
}

func TestDefault_ReturnsSensibleValues(t *testing.T) {
	bo := backoff.Default()

	if d := bo(0); d != 1*time.Second {
		t.Fatalf("attempt 0: expected 1s, got %v", d)
	}

	if d := bo(4); d != 16*time.Second {
		t.Fatalf("attempt 4: expected 16s, got %v", d)
	}

	// attempt 5: 32s → capped at 30s
	if d := bo(5); d != 30*time.Second {
		t.Fatalf("attempt 5: expected 30s (capped), got %v", d)
	}
}
