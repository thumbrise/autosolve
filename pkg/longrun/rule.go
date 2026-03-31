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

import (
	"context"
	"fmt"
	"log/slog"
)

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
	// Examples:
	//
	//	{Err: ErrTimeout}           // sentinel → errors.Is
	//	{Err: (*net.OpError)(nil)}  // pointer-to-type → errors.As
	Err error

	// MaxRetries limits consecutive retry attempts for this rule.
	//   0 (zero-value) → DefaultMaxRetries (3) — safe default.
	//  -1 (UnlimitedRetries) → no limit — explicit opt-in.
	//  >0 → exact retry count.
	MaxRetries int

	Backoff BackoffFunc
}

// TransientGroup creates N rules with identical MaxRetries and BackoffFunc.
// Each rule gets its own independent retry budget — failures of one error
// do not count toward the budget of another.
//
// Each error in errs must be a valid Err value (sentinel or typed nil pointer).
// See TransientRule.Err for details.
//
// Example:
//
//	longrun.TransientGroup(longrun.UnlimitedRetries, longrun.DefaultBackoff(),
//	    (*net.OpError)(nil),
//	    ErrFetchIssues,
//	    ErrStoreIssues,
//	)
func TransientGroup(maxRetries int, backoff BackoffFunc, errs ...error) []TransientRule {
	rules := make([]TransientRule, len(errs))
	for i, err := range errs {
		rules[i] = TransientRule{
			Err:        err,
			MaxRetries: maxRetries,
			Backoff:    backoff,
		}
	}

	return rules
}

// ruleFailureHandler implements failureHandler for a single TransientRule.
// Matches errors via Matcher (errors.Is/As), tracks attempts via AttemptStore,
// retries with BackoffFunc.
type ruleFailureHandler struct {
	rule     TransientRule
	matcher  Matcher
	key      string // opaque key for AttemptStore, e.g. "rule:0"
	attempts AttemptStore
	logger   *slog.Logger
}

func (h *ruleFailureHandler) Handle(ctx context.Context, err error) error {
	if !h.matcher.Match(err) {
		return errSkip
	}

	maxRetries := resolveMaxRetries(h.rule.MaxRetries)
	attempt := h.attempts.Increment(h.key)

	if maxRetries != UnlimitedRetries && attempt >= maxRetries {
		h.logger.ErrorContext(ctx, "max retries reached",
			slog.Any("error", err),
			slog.Int("max_retries", maxRetries),
		)

		return err
	}

	backoffDuration := h.rule.Backoff(attempt)

	h.logger.InfoContext(ctx, "transient error, retrying",
		slog.Int("attempt", attempt+1),
		slog.Any("error", err),
		slog.Any("backoff", backoffDuration),
	)

	sleepCtx(ctx, backoffDuration)

	return nil
}

// buildRuleHandlers validates rules and compiles them into failureHandlers.
// Panics on invalid rules (nil Err, unsupported Err type, nil Backoff).
func buildRuleHandlers(rules []TransientRule, attempts AttemptStore, logger *slog.Logger) []failureHandler {
	handlers := make([]failureHandler, len(rules))

	for i, r := range rules {
		if r.Backoff == nil {
			panic(fmt.Sprintf("longrun: TransientRule.Backoff must not be nil (rule Err: %v)", r.Err))
		}

		handlers[i] = &ruleFailureHandler{
			rule:     r,
			matcher:  NewMatcher(r.Err), // panics on nil or unsupported type
			key:      fmt.Sprintf("rule:%d", i),
			attempts: attempts,
			logger:   logger,
		}
	}

	return handlers
}
