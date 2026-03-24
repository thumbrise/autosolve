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

package longrun

// RuleTracker tracks retry attempts for a single TransientRule.
//
// Each rule has its own independent budget. The tracker is created
// internally by Task from TransientRule.MaxRetries.
type RuleTracker struct {
	max     int
	attempt int
}

// NewRuleTracker creates a tracker with the given max retries.
//
// MaxRetries semantics:
//
//	0 (zero-value) → DefaultMaxRetries (3).
//	-1 (UnlimitedRetries) → no limit.
//	>0 → exact limit.
func NewRuleTracker(maxRetries int) *RuleTracker {
	return &RuleTracker{
		max: resolveMaxRetries(maxRetries),
	}
}

// OnFailure records a failure and returns the 0-based attempt index
// and whether the caller is allowed to retry.
//
// Example with max=3:
//
//	1st call: attempt=0, ok=true
//	2nd call: attempt=1, ok=true
//	3rd call: attempt=2, ok=true
//	4th call: attempt=3, ok=false (budget exhausted)
func (rt *RuleTracker) OnFailure() (attempt int, ok bool) {
	if rt.max != UnlimitedRetries && rt.attempt >= rt.max {
		return rt.attempt, false
	}

	attempt = rt.attempt
	rt.attempt++

	return attempt, true
}

// Reset sets the attempt counter back to zero (e.g. after healthy progress).
func (rt *RuleTracker) Reset() {
	rt.attempt = 0
}

// Attempt returns the current attempt count.
func (rt *RuleTracker) Attempt() int {
	return rt.attempt
}

// Max returns the resolved max retries.
func (rt *RuleTracker) Max() int {
	return rt.max
}

func resolveMaxRetries(v int) int {
	switch {
	case v == UnlimitedRetries:
		return UnlimitedRetries
	case v <= 0:
		return DefaultMaxRetries
	default:
		return v
	}
}
