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
	logger   *slog.Logger
}

// NewOneShotTask creates a task that executes once.
// If rules is nil — no retries, any error is fatal.
// If rules is provided — transient errors are retried per their configuration.
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
		ruleStates = buildRuleStates(rules)
	}

	return &Task{
		name:     name,
		work:     work,
		shutdown: cfg.shutdown,
		interval: interval,
		timeout:  cfg.timeout,
		delay:    cfg.delay,
		rules:    ruleStates,
		logger:   logger,
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

	err := t.runWithPolicy(ctx)
	if err != nil {
		t.logger.ErrorContext(ctx, "permanent error", slog.Any("error", err))

		return err
	}

	t.logger.InfoContext(ctx, "stopped")

	return nil
}

// runWithPolicy implements the restart loop with backoff.
//
//	Task.Wait(ctx)
//	  └→ runWithPolicy (restart loop + backoff)
//	       └→ runLoop (ticker or one-shot)
//	            └→ runOnce (single invocation ± timeout)
func (t *Task) runWithPolicy(ctx context.Context) error {
	// Delay before first execution (if configured). Runs once, not on every retry.
	if t.delay > 0 {
		t.logger.DebugContext(ctx, "delaying first execution", slog.Any("delay", t.delay))

		timer := time.NewTimer(t.delay)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return nil //nolint:nilerr // context cancelled during delay
		case <-timer.C:
		}
	}

	for {
		err, hadProgress := t.runLoop(ctx)

		// --- success path ---
		if err == nil {
			t.resetAllRules()

			return nil
		}

		// Context done — not a task error.
		if ctx.Err() != nil {
			return nil //nolint:nilerr // context termination is not a task failure
		}

		// --- failure path ---
		if retryErr := t.handleFailure(ctx, err, hadProgress); retryErr != nil {
			return retryErr
		}
	}
}

// handleFailure processes an error: finds a matching rule, checks retry budget,
// waits for backoff. Returns nil if the caller should retry, or the error to stop.
func (t *Task) handleFailure(ctx context.Context, err error, hadProgress bool) error {
	rs := t.findMatchingRule(err)
	if rs == nil {
		// No matching rule → permanent error.
		return err
	}

	if hadProgress {
		t.resetAllRules()
	}

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
}

// runLoop runs the ticker loop (interval > 0) or a single invocation (one-shot).
// The second return value (hadProgress) is true when at least one invocation
// of the work function succeeded before the loop returned an error.
func (t *Task) runLoop(ctx context.Context) (error, bool) {
	if t.interval <= 0 {
		return t.runOnce(ctx), false
	}

	// Interval mode: run immediately, then on every tick.
	hadProgress := false

	if err := t.runOnce(ctx); err != nil {
		return err, false
	}

	hadProgress = true

	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, hadProgress
		case <-ticker.C:
			if err := t.runOnce(ctx); err != nil {
				return err, hadProgress
			}

			hadProgress = true
		}
	}
}

// runOnce executes the work function once, optionally with a per-invocation timeout.
func (t *Task) runOnce(ctx context.Context) error {
	if t.timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, t.timeout)
		defer cancel()

		t.logger.DebugContext(ctx, "timeout applied", slog.Any("timeout", t.timeout))
	}

	return t.work(ctx)
}
