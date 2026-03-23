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

import "errors"

// ---------------------------------------------------------------------------
// RestartStrategy — what to do after runLoop completes.
// ---------------------------------------------------------------------------

// RestartStrategy decides whether the task loop should restart.
type RestartStrategy interface {
	ShouldRestart(err error) bool
}

// NeverRestart stops the task after any completion (success or failure).
type NeverRestart struct{}

func (NeverRestart) ShouldRestart(error) bool { return false }

// RestartOnFailure restarts only when the task returned a non-nil error.
type RestartOnFailure struct{}

func (RestartOnFailure) ShouldRestart(err error) bool { return err != nil }

// ---------------------------------------------------------------------------
// ErrorClassifier — transient vs permanent.
// ---------------------------------------------------------------------------

// ErrorClassifier decides whether an error is transient (retryable).
type ErrorClassifier interface {
	IsTransient(err error) bool
}

// AllPermanent treats every error as permanent.
type AllPermanent struct{}

func (AllPermanent) IsTransient(error) bool { return false }

// WhitelistClassifier treats only listed errors (matched via errors.Is) as transient.
// All other errors are considered permanent.
type WhitelistClassifier struct {
	Errors []error
}

// NewWhitelistClassifier creates a WhitelistClassifier from the given sentinel errors.
func NewWhitelistClassifier(errs ...error) WhitelistClassifier {
	return WhitelistClassifier{Errors: errs}
}

func (w WhitelistClassifier) IsTransient(err error) bool {
	for _, te := range w.Errors {
		if errors.Is(err, te) {
			return true
		}
	}

	return false
}

// ---------------------------------------------------------------------------
// AttemptTracker — retry budget management.
// ---------------------------------------------------------------------------

// AttemptTracker tracks retry attempts and decides whether more retries are allowed.
type AttemptTracker interface {
	// OnFailure returns the 0-based attempt index to use for backoff calculation
	// and whether the task is allowed to retry.
	//
	// Example with maxRetries=3:
	//   1st call: attempt=0, ok=true
	//   2nd call: attempt=1, ok=true
	//   3rd call: attempt=2, ok=true
	//   4th call: attempt=3, ok=false (budget exhausted)
	OnFailure() (attempt int, ok bool)
	// Reset sets the attempt counter back to zero (e.g. after healthy progress).
	Reset()
}

// UnlimitedAttempts never exhausts the retry budget.
type UnlimitedAttempts struct {
	attempt int
}

// NewUnlimitedAttempts creates an AttemptTracker with no retry limit.
func NewUnlimitedAttempts() *UnlimitedAttempts {
	return &UnlimitedAttempts{}
}

func (u *UnlimitedAttempts) OnFailure() (int, bool) {
	attempt := u.attempt
	u.attempt++

	return attempt, true
}

// Reset sets the attempt counter back to zero.
func (u *UnlimitedAttempts) Reset() { u.attempt = 0 }

// LimitedAttempts stops retrying after MaxRetries consecutive failures.
type LimitedAttempts struct {
	MaxRetries int
	attempt    int
}

// NewLimitedAttempts creates an AttemptTracker that allows at most maxRetries retries.
func NewLimitedAttempts(maxRetries int) *LimitedAttempts {
	return &LimitedAttempts{MaxRetries: maxRetries}
}

func (l *LimitedAttempts) OnFailure() (int, bool) {
	if l.MaxRetries > 0 && l.attempt >= l.MaxRetries {
		return l.attempt, false
	}

	attempt := l.attempt
	l.attempt++

	return attempt, true
}

// Reset sets the attempt counter back to zero.
func (l *LimitedAttempts) Reset() { l.attempt = 0 }
