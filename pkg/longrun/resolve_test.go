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
	"errors"
	"testing"

	"github.com/thumbrise/autosolve/pkg/longrun"
)

// ---------------------------------------------------------------------------
// ResolveRestartStrategy
// ---------------------------------------------------------------------------

func TestResolveRestartStrategy_Never(t *testing.T) {
	s := longrun.ResolveRestartStrategy(longrun.Never)

	if _, ok := s.(longrun.NeverRestart); !ok {
		t.Errorf("Never → %T, want NeverRestart", s)
	}
}

func TestResolveRestartStrategy_Always(t *testing.T) {
	s := longrun.ResolveRestartStrategy(longrun.Always)

	if _, ok := s.(longrun.AlwaysRestart); !ok {
		t.Errorf("Always → %T, want AlwaysRestart", s)
	}
}

func TestResolveRestartStrategy_OnFailure(t *testing.T) {
	s := longrun.ResolveRestartStrategy(longrun.OnFailure)

	if _, ok := s.(longrun.RestartOnFailure); !ok {
		t.Errorf("OnFailure → %T, want RestartOnFailure", s)
	}
}

func TestResolveRestartStrategy_UnknownFallsBackToNever(t *testing.T) {
	s := longrun.ResolveRestartStrategy(longrun.RestartPolicy(99))

	if _, ok := s.(longrun.NeverRestart); !ok {
		t.Errorf("unknown policy → %T, want NeverRestart", s)
	}
}

// ---------------------------------------------------------------------------
// ResolveErrorClassifier
// ---------------------------------------------------------------------------

func TestResolveErrorClassifier_Nil(t *testing.T) {
	c := longrun.ResolveErrorClassifier(nil)

	if _, ok := c.(longrun.AllPermanent); !ok {
		t.Errorf("nil → %T, want AllPermanent", c)
	}
}

func TestResolveErrorClassifier_Empty(t *testing.T) {
	c := longrun.ResolveErrorClassifier([]error{})

	if _, ok := c.(longrun.AllPermanent); !ok {
		t.Errorf("[] → %T, want AllPermanent", c)
	}
}

func TestResolveErrorClassifier_WithErrors(t *testing.T) {
	sentinel := errors.New("net")
	c := longrun.ResolveErrorClassifier([]error{sentinel})

	if _, ok := c.(longrun.WhitelistClassifier); !ok {
		t.Fatalf("[net] → %T, want WhitelistClassifier", c)
	}

	if !c.IsTransient(sentinel) {
		t.Error("sentinel should be transient")
	}

	if c.IsTransient(errors.New("other")) {
		t.Error("unknown error should be permanent")
	}
}

// ---------------------------------------------------------------------------
// ResolveAttemptTracker
// ---------------------------------------------------------------------------

func TestResolveAttemptTracker_Zero(t *testing.T) {
	a := longrun.ResolveAttemptTracker(0)

	if _, ok := a.(*longrun.UnlimitedAttempts); !ok {
		t.Errorf("0 → %T, want *UnlimitedAttempts", a)
	}
}

func TestResolveAttemptTracker_Negative(t *testing.T) {
	a := longrun.ResolveAttemptTracker(-1)

	if _, ok := a.(*longrun.UnlimitedAttempts); !ok {
		t.Errorf("-1 → %T, want *UnlimitedAttempts", a)
	}
}

func TestResolveAttemptTracker_Positive(t *testing.T) {
	a := longrun.ResolveAttemptTracker(5)

	la, ok := a.(*longrun.LimitedAttempts)
	if !ok {
		t.Fatalf("5 → %T, want *LimitedAttempts", a)
	}

	if la.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", la.MaxRetries)
	}
}
