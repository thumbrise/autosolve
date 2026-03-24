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
	"testing"
	"time"

	"github.com/thumbrise/autosolve/pkg/longrun"
)

func TestBackoffConfig_Wait_RespectsContext(t *testing.T) {
	cfg := longrun.BackoffConfig{
		Initial:    10 * time.Second,
		Max:        10 * time.Second,
		Multiplier: 2.0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	start := time.Now()
	err := cfg.Wait(ctx, 0)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from cancelled context")
	}

	if elapsed > 100*time.Millisecond {
		t.Fatalf("wait should have returned immediately, took %v", elapsed)
	}
}

func TestBackoffConfig_Wait_CompletesOnTime(t *testing.T) {
	cfg := longrun.BackoffConfig{
		Initial:    10 * time.Millisecond,
		Max:        100 * time.Millisecond,
		Multiplier: 2.0,
	}

	ctx := context.Background()

	start := time.Now()
	err := cfg.Wait(ctx, 0)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if elapsed < 5*time.Millisecond {
		t.Fatalf("wait returned too fast: %v", elapsed)
	}
}

func TestBackoffConfig_Duration_CapsAtMax(t *testing.T) {
	cfg := longrun.BackoffConfig{
		Initial:    1 * time.Second,
		Max:        5 * time.Second,
		Multiplier: 10.0,
	}

	// attempt 0: 1s * 10^0 = 1s
	d0 := cfg.Duration(0)
	if d0 != 1*time.Second {
		t.Fatalf("attempt 0: expected 1s, got %v", d0)
	}

	// attempt 1: 1s * 10^1 = 10s → capped at 5s
	d1 := cfg.Duration(1)
	if d1 != 5*time.Second {
		t.Fatalf("attempt 1: expected 5s (capped), got %v", d1)
	}
}

func TestBackoffConfig_Duration_NoMaxNoCap(t *testing.T) {
	cfg := longrun.BackoffConfig{
		Initial:    1 * time.Second,
		Max:        0, // no cap
		Multiplier: 2.0,
	}

	// attempt 3: 1s * 2^3 = 8s — no cap
	d := cfg.Duration(3)
	if d != 8*time.Second {
		t.Fatalf("expected 8s, got %v", d)
	}
}
