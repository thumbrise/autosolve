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
	"time"

	"github.com/thumbrise/autosolve/pkg/longrun"
)

func TestBackoff_Constructor(t *testing.T) {
	cfg := longrun.Backoff(2*time.Second, 2*time.Minute)

	if cfg.Initial != 2*time.Second {
		t.Fatalf("expected Initial 2s, got %v", cfg.Initial)
	}

	if cfg.Max != 2*time.Minute {
		t.Fatalf("expected Max 2m, got %v", cfg.Max)
	}

	if cfg.Multiplier != 2.0 {
		t.Fatalf("expected Multiplier 2.0, got %v", cfg.Multiplier)
	}
}

func TestBackoff_ExponentialGrowth(t *testing.T) {
	cfg := longrun.Backoff(2*time.Second, 2*time.Minute)

	// attempt 0: 2s * 2^0 = 2s
	if d := cfg.Duration(0); d != 2*time.Second {
		t.Fatalf("attempt 0: expected 2s, got %v", d)
	}

	// attempt 1: 2s * 2^1 = 4s
	if d := cfg.Duration(1); d != 4*time.Second {
		t.Fatalf("attempt 1: expected 4s, got %v", d)
	}

	// attempt 2: 2s * 2^2 = 8s
	if d := cfg.Duration(2); d != 8*time.Second {
		t.Fatalf("attempt 2: expected 8s, got %v", d)
	}

	// attempt 10: 2s * 2^10 = 2048s → capped at 2m
	if d := cfg.Duration(10); d != 2*time.Minute {
		t.Fatalf("attempt 10: expected 2m (capped), got %v", d)
	}
}

func TestPolicy_ZeroRetries_IsUnlimited(t *testing.T) {
	p := longrun.Policy{}

	// Zero-value Retries means unlimited for baseline policies.
	// This is different from TransientRule where 0 → DefaultMaxRetries(3).
	if p.Retries != 0 {
		t.Fatalf("expected zero-value Retries, got %d", p.Retries)
	}
}

func TestBaseline_DefaultNil_IsZeroValue(t *testing.T) {
	b := longrun.Baseline{}

	// Zero-value Baseline has Default == nil → unknown errors are permanent.
	if b.Default != nil {
		t.Fatal("expected nil Default in zero-value Baseline")
	}

	// Zero-value Baseline has Classify == nil → no application classifier.
	if b.Classify != nil {
		t.Fatal("expected nil Classify in zero-value Baseline")
	}

	// Zero-value Baseline has empty Policies map.
	if len(b.Policies) != 0 {
		t.Fatalf("expected empty Policies, got %d", len(b.Policies))
	}
}

func TestNewBaseline_SetsNodeAndService(t *testing.T) {
	node := longrun.Policy{Backoff: longrun.Backoff(1*time.Second, 1*time.Minute)}
	service := longrun.Policy{Backoff: longrun.Backoff(2*time.Second, 2*time.Minute)}

	b := longrun.NewBaseline(node, service, nil)

	if len(b.Policies) != 2 {
		t.Fatalf("expected 2 policies, got %d", len(b.Policies))
	}

	if b.Default != nil {
		t.Fatal("expected nil Default from NewBaseline")
	}
}

func TestNewBaselineDegraded_SetsDefault(t *testing.T) {
	node := longrun.Policy{Backoff: longrun.Backoff(1*time.Second, 1*time.Minute)}
	service := longrun.Policy{Backoff: longrun.Backoff(2*time.Second, 2*time.Minute)}
	degraded := longrun.Policy{Backoff: longrun.Backoff(5*time.Second, 5*time.Minute)}

	b := longrun.NewBaselineDegraded(node, service, degraded, nil)

	if len(b.Policies) != 2 {
		t.Fatalf("expected 2 policies, got %d", len(b.Policies))
	}

	if b.Default == nil {
		t.Fatal("expected non-nil Default from NewBaselineDegraded")
	}

	if b.Default.Backoff.Initial != 5*time.Second {
		t.Fatalf("expected Default.Backoff.Initial 5s, got %v", b.Default.Backoff.Initial)
	}
}
