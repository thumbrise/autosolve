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
	"errors"
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
	handlers []failureHandler
	logger   *slog.Logger
	attempts AttemptStore
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
// Panics if any rule has nil Err, unsupported Err type, or nil Backoff.
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
// Panics if any rule has nil Err, unsupported Err type, or nil Backoff.
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

	store := cfg.attemptStore
	if store == nil {
		store = NewMemoryStore()
	}

	var handlers []failureHandler

	if len(rules) > 0 {
		for _, r := range rules {
			if r.MaxRetries == 0 {
				logger.Warn("TransientRule.MaxRetries is 0 (zero-value), using DefaultMaxRetries",
					slog.Int("resolved", DefaultMaxRetries),
					slog.Any("rule", r),
				)
			}
		}

		handlers = buildRuleHandlers(rules, store, logger)
	}

	return &Task{
		name:     name,
		work:     work,
		shutdown: cfg.shutdown,
		interval: interval,
		timeout:  cfg.timeout,
		delay:    cfg.delay,
		handlers: handlers,
		logger:   logger,
		attempts: store,
	}
}

// Wait runs the task to completion, respecting the configured retry policy,
// backoff and interval. It blocks until the task finishes or ctx is cancelled.
func (t *Task) Wait(ctx context.Context) error {
	t.logger.InfoContext(ctx, "started",
		slog.Any("interval", t.interval),
		slog.Any("timeout", t.timeout),
		slog.Any("delay", t.delay),
		slog.Int("handlers", len(t.handlers)),
	)

	err := t.restartLoop(ctx)
	if err != nil {
		t.logger.ErrorContext(ctx, "permanent error", slog.Any("error", err))

		return err
	}

	t.logger.InfoContext(ctx, "stopped")

	return nil
}

// handleFailure processes an error through the handler pipeline.
// Returns nil if the caller should retry, or the error to stop.
//
// Handlers are checked in order:
//  1. Per-task TransientRules (explicit error matching via errors.Is/As)
//  2. Baseline classification (transport + user classifier + degraded mode)
//
// Each handler returns:
//   - errSkip → not my error, try next handler
//   - nil → handled, retry
//   - other error → permanent, stop
//
// If no handler matches → permanent error.
//
// Retry budget semantics: when an interval task completes a successful tick
// (hadProgress=true), all attempt counters reset to zero — so intermittent
// failures separated by successful ticks never accumulate toward the limit.
func (t *Task) handleFailure(ctx context.Context, err error, hadProgress bool) error {
	if hadProgress {
		t.attempts.Reset()
	}

	for _, h := range t.handlers {
		result := h.Handle(ctx, err)
		if !errors.Is(result, errSkip) {
			return result
		}
	}

	return err // no handler matched → permanent
}
