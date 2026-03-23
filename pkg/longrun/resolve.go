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

// ResolveRestartStrategy maps a RestartPolicy enum to a concrete RestartStrategy.
func ResolveRestartStrategy(p RestartPolicy) RestartStrategy {
	switch p {
	case OnFailure:
		return RestartOnFailure{}
	case Never:
		return NeverRestart{}
	default:
		return NeverRestart{}
	}
}

// ResolveErrorClassifier builds an ErrorClassifier from the transient errors whitelist.
// Empty list → AllPermanent (every error stops the task).
func ResolveErrorClassifier(transientErrors []error) ErrorClassifier {
	if len(transientErrors) == 0 {
		return AllPermanent{}
	}

	return NewWhitelistClassifier(transientErrors...)
}

// ResolveAttemptTracker builds an AttemptTracker from MaxRetries.
//
//	UnlimitedRetries (-1) → unlimited retries.
//	0 (zero-value)        → DefaultMaxRetries (3).
//	>0                    → exact limit.
func ResolveAttemptTracker(maxRetries int) AttemptTracker {
	switch {
	case maxRetries == UnlimitedRetries:
		return NewUnlimitedAttempts()
	case maxRetries <= 0:
		return NewLimitedAttempts(DefaultMaxRetries)
	default:
		return NewLimitedAttempts(maxRetries)
	}
}
