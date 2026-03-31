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
	"log/slog"
	"time"
)

// WorkFunc is the function that performs the actual work of a task.
type WorkFunc func(ctx context.Context) error

// Task is a self-contained unit of work with interval, retry and backoff support.
// It can be used standalone (via Wait) or managed by a Runner.
//
// Task is NOT safe for concurrent use — call Wait from a single goroutine.
// Runner handles this automatically (one goroutine per task).
type Task struct {
	name     string
	work     WorkFunc
	shutdown ShutdownFunc
	interval time.Duration
	timeout  time.Duration
	delay    time.Duration
	rules    []ruleState
	baseline *Baseline // set by Runner.Add, nil when standalone
	logger   *slog.Logger

	// baselineAttempts tracks consecutive retry attempts per baseline category.
	// Keyed by ErrorCategory — supports both predefined and user-defined categories.
	// Used for exponential backoff: attempt N → Backoff.Duration(N).
	// Reset to zero on successful tick (hadProgress=true) together with rule trackers.
	//
	// Example: WiFi drops, 3 consecutive Node errors:
	//   attempt 0 → 2s, attempt 1 → 4s, attempt 2 → 8s
	//   WiFi recovers, tick succeeds → all counters reset to 0.
	baselineAttempts map[ErrorCategory]int
}

// NewOneShotTask creates a task that executes once.
// If rules is nil — no retries, any error is fatal.
// If rules is provided — transient errors are retried per their configuration.
//
// Each TransientRule binds an error to its own retry budget and backoff curve.
// TransientRule.MaxRetries limits consecutive failures for that rule — the budget
// is never reset mid-execution for one-shot tasks.
//
// Panics if work is nil.
// Panics if any rule has nil Err, unsupported Err type, or Backoff.Initial <= 0.
func NewOneShotTask(name string, work WorkFunc, rules []TransientRule, opts ...Option) *Task {
	return newTask(name, 0, work, rules, opts)
}

// NewIntervalTask creates a task that runs on a ticker loop.
// If rules is nil — any error kills the task.
// If rules is provided — transient errors are retried per their configuration,
// permanent errors (no matching rule) kill the task.
//
// Each TransientRule binds an error to its own retry budget and backoff curve.
// TransientRule.MaxRetries limits consecutive failures for that rule. When a tick
// completes successfully, all rule trackers reset — so intermittent failures
// separated by successful ticks never accumulate toward MaxRetries.
//
// Panics if work is nil or interval <= 0.
// Panics if any rule has nil Err, unsupported Err type, or Backoff.Initial <= 0.
func NewIntervalTask(name string, interval time.Duration, work WorkFunc, rules []TransientRule, opts ...Option) *Task {
	if interval <= 0 {
		panic("longrun.NewIntervalTask: interval must be > 0")
	}

	return newTask(name, interval, work, rules, opts)
}

func newTask(name string, interval time.Duration, work WorkFunc, rules []TransientRule, opts []Option) *Task {
	if work == nil {
		panic("longrun: work function must not be nil")
	}

	cfg := &taskConfig{}
	for _, o := range opts {
		o(cfg)
	}

	logger := cfg.logger
	if logger == nil {
		logger = slog.Default()
	}

	logger = logger.With(slog.String("task", name))

	var ruleStates []ruleState

	if len(rules) > 0 {
		for _, r := range rules {
			if r.MaxRetries == 0 {
				logger.Warn("TransientRule.MaxRetries is 0 (zero-value), using DefaultMaxRetries",
					slog.Int("resolved", DefaultMaxRetries),
					slog.Any("rule_err", r.Err),
				)
			}
		}

		ruleStates = buildRuleStates(rules)
	}

	return &Task{
		name:             name,
		work:             work,
		shutdown:         cfg.shutdown,
		interval:         interval,
		timeout:          cfg.timeout,
		delay:            cfg.delay,
		rules:            ruleStates,
		logger:           logger,
		baselineAttempts: make(map[ErrorCategory]int),
	}
}

// Wait runs the task to completion, respecting the configured retry policy,
// backoff and interval. It blocks until the task finishes or ctx is cancelled.
func (t *Task) Wait(ctx context.Context) error {
	t.logger.InfoContext(ctx, "started",
		slog.Any("interval", t.interval),
		slog.Any("timeout", t.timeout),
		slog.Any("delay", t.delay),
		slog.Int("rules", len(t.rules)),
	)

	err := t.restartLoop(ctx)
	if err != nil {
		t.logger.ErrorContext(ctx, "permanent error", slog.Any("error", err))

		return err
	}

	t.logger.InfoContext(ctx, "stopped")

	return nil
}

// handleFailure processes an error through a linear classification pipeline.
// Returns nil if the caller should retry, or the error to stop.
//
// Pipeline:
//
//  1. Per-task TransientRules (legacy, explicit error matching)
//  2. Baseline classification (if baseline is set):
//     a. Built-in transport classify (net.OpError, timeout → Node)
//     b. User classifier via Baseline.Classify (apierr interfaces → Service)
//     c. Not classified → Unknown ->
//     Unknown + Default != nil → retry with Default policy (LOUD log)
//     Unknown + Default == nil → permanent error
//
// Retry budget semantics: when an interval task completes a successful tick
// (hadProgress=true), all rule trackers reset to zero — so intermittent failures
// separated by successful ticks never accumulate toward the retry limit.
func (t *Task) handleFailure(ctx context.Context, err error, hadProgress bool) error {
	if hadProgress {
		t.resetAllRules()
	}

	// Step 1: per-task TransientRules (explicit error matching).
	if rs := t.findMatchingRule(err); rs != nil {
		return t.retryWithRule(ctx, err, rs)
	}

	// Step 2: baseline classification pipeline.
	if t.baseline == nil {
		return err // no baseline, no matching rule → permanent.
	}

	return t.handleBaselineFailure(ctx, err)
}

// retryWithRule retries using a per-task TransientRule (legacy path).
func (t *Task) retryWithRule(ctx context.Context, err error, rs *ruleState) error {
	attempt, canRetry := rs.tracker.OnFailure()
	if !canRetry {
		t.logger.ErrorContext(ctx, "max retries reached",
			slog.Any("error", err),
			slog.Int("max_retries", rs.tracker.Max()),
		)

		return err
	}

	backoffDuration := rs.rule.Backoff.Duration(attempt)

	t.logger.InfoContext(ctx, "transient error, retrying",
		slog.Int("attempt", attempt+1),
		slog.Any("error", err),
		slog.Any("backoff", backoffDuration),
	)

	if waitErr := rs.rule.Backoff.Wait(ctx, attempt); waitErr != nil {
		return nil //nolint:nilerr // context cancelled during backoff, next iteration handles it
	}

	return nil
}

func (t *Task) findMatchingRule(err error) *ruleState {
	for i := range t.rules {
		if t.rules[i].matcher.Match(err) {
			return &t.rules[i]
		}
	}

	return nil
}

func (t *Task) resetAllRules() {
	for i := range t.rules {
		t.rules[i].tracker.Reset()
	}

	t.baselineAttempts = make(map[ErrorCategory]int)
}
