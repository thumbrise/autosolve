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

import "fmt"

// TransientRule binds an error to its retry settings.
// Different errors can have different retry budgets and backoff curves.
//
// The Err field accepts two forms:
//   - error value (sentinel): matched via errors.Is
//   - *T where T implements error: matched via errors.As
//
// Examples:
//
//	{Err: ErrTimeout}           // sentinel → errors.Is
//	{Err: (*net.OpError)(nil)}  // pointer-to-type → errors.As
type TransientRule struct {
	// Err is the error to match.
	// Must be an error value (for errors.Is) or a pointer to an error type (for errors.As).
	// Passing nil or an unsupported type panics at construction time.
	Err any

	// MaxRetries limits consecutive retry attempts for this rule.
	//   0 (zero-value) → DefaultMaxRetries (3) — safe default.
	//  -1 (UnlimitedRetries) → no limit — explicit opt-in.
	//  >0 → exact retry count.
	MaxRetries int

	Backoff BackoffConfig
}

// ruleState is the internal, mutable representation of a TransientRule.
// TransientRule itself is a pure config value from the caller.
type ruleState struct {
	rule    TransientRule
	matcher Matcher
	tracker *RuleTracker
}

// buildRuleStates validates rules and compiles them into ruleStates.
// Panics on invalid rules (nil Err, unsupported Err type, zero Initial backoff).
func buildRuleStates(rules []TransientRule) []ruleState {
	states := make([]ruleState, len(rules))

	for i, r := range rules {
		if r.Backoff.Initial <= 0 {
			panic(fmt.Sprintf("longrun: TransientRule.Backoff.Initial must be > 0, got %v (rule Err: %v)", r.Backoff.Initial, r.Err))
		}

		if r.Backoff.Multiplier <= 0 {
			panic(fmt.Sprintf("longrun: TransientRule.Backoff.Multiplier must be > 0, got %v (rule Err: %v)", r.Backoff.Multiplier, r.Err))
		}

		states[i] = ruleState{
			rule:    r,
			matcher: NewMatcher(r.Err), // panics on nil or unsupported type
			tracker: NewRuleTracker(r.MaxRetries),
		}
	}

	return states
}
