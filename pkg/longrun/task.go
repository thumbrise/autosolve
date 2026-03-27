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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
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
	// Indexed by ErrorCategory (CategoryUnknown=0, CategoryNode=1, CategoryService=2).
	// Used for exponential backoff: attempt N → Backoff.Duration(N).
	// Reset to zero on successful tick (hadProgress=true) together with rule trackers.
	//
	// Example: WiFi drops, 3 consecutive Node errors:
	//   attempt 0 → 2s, attempt 1 → 4s, attempt 2 → 8s
	//   WiFi recovers, tick succeeds → all counters reset to 0.
	baselineAttempts [3]int
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
			return nil
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
			return nil
		}

		// --- failure path ---
		if retryErr := t.handleFailure(ctx, err, hadProgress); retryErr != nil {
			return retryErr
		}
	}
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
//     Unknown + Degraded != nil → retry with Degraded policy (LOUD log)
//     Unknown + Degraded == nil → permanent error
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

// handleBaselineFailure classifies err via baseline and retries accordingly.
func (t *Task) handleBaselineFailure(ctx context.Context, err error) error {
	class, policy := t.classifyWithBaseline(err)

	if policy == nil {
		// Unknown + no Degraded → permanent error.
		return err
	}

	isDegraded := class.Category == CategoryUnknown
	if isDegraded {
		t.logger.ErrorContext(ctx, "DEGRADED: unknown error, retrying with degraded policy",
			slog.Any("error", err),
		)
	}

	return t.retryWithPolicy(ctx, err, policy, class.Category, class.WaitDuration, isDegraded)
}

// classifyWithBaseline runs the classification pipeline and returns the
// ErrorClass and the matching Policy. Returns nil policy when the error
// is unknown and Baseline.Degraded is nil.
func (t *Task) classifyWithBaseline(err error) (*ErrorClass, *Policy) {
	// [1] Built-in transport classify.
	if class := ClassifyTransport(err); class != nil {
		return class, &t.baseline.Node
	}

	// [2] User classifier.
	if t.baseline.Classify != nil {
		if class := t.baseline.Classify(err); class != nil {
			p := t.baseline.Degraded

			switch class.Category {
			case CategoryNode:
				p = &t.baseline.Node
			case CategoryService:
				p = &t.baseline.Service
			case CategoryUnknown:
				// CategoryUnknown or future categories → Degraded policy.
				// Preserves class (including WaitDuration) from the classifier.
				p = t.baseline.Degraded
			}

			return class, p
		}
	}

	// [3] Unknown — no classifier matched.
	unknown := &ErrorClass{Category: CategoryUnknown}

	return unknown, t.baseline.Degraded // Degraded may be nil → permanent
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

// retryWithPolicy retries using a baseline Policy.
// category is used to track per-category attempt count for exponential backoff.
// When waitOverride > 0, sleeps exactly that duration instead of backoff.
func (t *Task) retryWithPolicy(ctx context.Context, err error, p *Policy, category ErrorCategory, waitOverride time.Duration, isDegraded bool) error {
	//nolint:godox // retry budget tracking deferred — baseline policies retry indefinitely for now (zero-value = unlimited).
	// TODO: track per-policy retry budget (Policy.Retries). See #121.
	attempt := t.baselineAttempts[category]
	t.baselineAttempts[category]++

	categoryLabel := categoryName(category)

	taskAttr := attribute.String("task", t.name)
	categoryAttr := attribute.String("category", categoryLabel)

	metricBaselineRetryTotal.Add(ctx, 1, metric.WithAttributes(taskAttr, categoryAttr))

	if isDegraded {
		metricDegradedTotal.Add(ctx, 1, metric.WithAttributes(taskAttr))
	}

	level := slog.LevelInfo
	if isDegraded {
		level = slog.LevelError
	}

	var waitDur time.Duration

	if waitOverride > 0 {
		waitDur = waitOverride

		t.logger.Log(ctx, level, "retrying after explicit wait",
			slog.Any("error", err),
			slog.Any("wait", waitDur),
			slog.Int("attempt", attempt+1),
		)
	} else {
		waitDur = p.Backoff.Duration(attempt)

		t.logger.Log(ctx, level, "retrying with backoff",
			slog.Any("error", err),
			slog.Any("backoff", waitDur),
			slog.Int("attempt", attempt+1),
		)
	}

	start := time.Now()
	result := t.waitDuration(ctx, waitDur)

	if isDegraded {
		metricDegradedDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(taskAttr))
	}

	return result
}

// categoryName returns a human-readable label for metrics and logs.
func categoryName(c ErrorCategory) string {
	switch c {
	case CategoryUnknown:
		return "degraded"
	case CategoryNode:
		return "node"
	case CategoryService:
		return "service"
	}

	return "degraded"
}

// waitDuration sleeps for d or until ctx is cancelled.
// Returns nil on successful wait (caller should retry),
// nil on context cancellation (next iteration handles it).
func (t *Task) waitDuration(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil
	case <-timer.C:
		return nil
	}
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

	t.baselineAttempts = [3]int{}
}

// runLoop runs the ticker loop (interval > 0) or a single invocation (one-shot).
// The second return value (hadProgress) is true when at least one invocation
// of the work function succeeded before the loop returned an error.
//
// Interval mode always runs work immediately, then on every tick. When a transient
// error triggers a retry via handleFailure, runWithPolicy re-enters runLoop from
// scratch: work runs immediately again, then a new ticker starts. This means the
// ticker resets on each retry — the interval between the immediate retry and the
// next tick is a full interval period, not the remainder of the previous one.
func (t *Task) runLoop(ctx context.Context) (error, bool) {
	if t.interval <= 0 {
		return t.runOnce(ctx), false
	}

	// Interval mode: run immediately, then on every tick.
	if err := t.runOnce(ctx); err != nil {
		return err, false
	}

	hadProgress := true

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
	ctx, span := otel.Tracer("github.com/thumbrise/autosolve/pkg/longrun").Start(ctx, t.name)
	defer span.End()

	if t.timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, t.timeout)
		defer cancel()

		t.logger.DebugContext(ctx, "timeout applied", slog.Any("timeout", t.timeout))
	}

	t.logger.DebugContext(ctx, "iteration start", slog.Any("interval", t.interval))

	startTime := time.Now()

	err := t.work(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	duration := time.Since(startTime)
	t.logger.DebugContext(ctx, "iteration finished", slog.Any("duration", duration))

	return err
}
