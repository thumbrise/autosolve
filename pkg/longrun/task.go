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

// ShutdownFunc is called during graceful shutdown.
type ShutdownFunc func(ctx context.Context) error

// RestartPolicy determines when a task should be restarted after completion.
type RestartPolicy int

const (
	Never     RestartPolicy = iota // do not restart (default)
	Always                         // restart on success and failure
	OnFailure                      // restart only on failure
)

// TaskOptions configures the behaviour of a Task.
//
// By default all errors are permanent — a single failure stops the task.
// To enable retries, set Restart to OnFailure or Always and list the
// errors that should be retried in TransientErrors (whitelist).
// Only errors matching via errors.Is are considered transient;
// everything else remains permanent and stops the task immediately.
type TaskOptions struct {
	Interval        time.Duration // 0 = one-shot
	SkipInitialRun  bool          // default false = run immediately
	Restart         RestartPolicy // default Never
	Backoff         BackoffConfig
	Timeout         time.Duration // per-invocation, 0 = none
	TransientErrors []error       // whitelist; empty = all errors are permanent
	Logger          *slog.Logger  // nil = slog.Default()
}

// Task is a self-contained unit of work with interval, retry and backoff support.
// It can be used standalone (via Wait) or managed by a Runner.
//
// Task is NOT safe for concurrent use — call Wait from a single goroutine.
// Runner handles this automatically (one goroutine per task).
type Task struct {
	Name     string
	Work     WorkFunc
	Shutdown ShutdownFunc
	Options  TaskOptions

	restart    RestartStrategy
	classifier ErrorClassifier
	attempts   AttemptTracker
	logger     *slog.Logger
}

// NewTask creates a Task with the given name, work function and options.
// Panics if work is nil.
func NewTask(name string, work WorkFunc, opts TaskOptions) *Task {
	if work == nil {
		panic("longrun.NewTask: work function is nil")
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	logger = logger.With(slog.String("task", name))

	return &Task{
		Name:       name,
		Work:       work,
		Options:    opts,
		restart:    ResolveRestartStrategy(opts.Restart),
		classifier: ResolveErrorClassifier(opts.TransientErrors),
		attempts:   ResolveAttemptTracker(opts.Backoff.MaxRetries),
		logger:     logger,
	}
}

// Wait runs the task to completion, respecting the configured restart policy,
// backoff and interval.  It blocks until the task finishes or ctx is cancelled.
func (t *Task) Wait(ctx context.Context) error {
	t.logger.InfoContext(ctx, "started",
		slog.Any("interval", t.Options.Interval),
		slog.Any("restart", t.Options.Restart),
		slog.Any("timeout", t.Options.Timeout),
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
// Each decision is delegated to a strategy resolved at construction time,
// so this method contains no policy-specific branching.
func (t *Task) runWithPolicy(ctx context.Context) error {
	for {
		err, hadProgress := t.runLoop(ctx)

		// --- success path ---
		if err == nil {
			t.attempts.Reset()

			if ctx.Err() != nil {
				return nil //nolint:nilerr // context cancelled, clean shutdown
			}

			if !t.restart.ShouldRestart(nil) {
				return nil
			}

			t.logger.InfoContext(ctx, "completed, restarting")

			continue
		}

		// Context cancelled — not a task error.
		if errors.Is(err, context.Canceled) {
			return nil
		}

		// --- failure path ---
		if retryErr := t.handleFailure(ctx, err, hadProgress); retryErr != nil {
			return retryErr
		}
	}
}

// handleFailure processes a transient error: classifies it, checks retry budget,
// waits for backoff.  Returns nil if the caller should retry, or the error to stop.
func (t *Task) handleFailure(ctx context.Context, err error, hadProgress bool) error {
	if !t.classifier.IsTransient(err) || !t.restart.ShouldRestart(err) {
		return err
	}

	if hadProgress {
		t.attempts.Reset()
	}

	attempt, canRetry := t.attempts.OnFailure()
	if !canRetry {
		t.logger.ErrorContext(ctx, "max retries reached", slog.Any("error", err))

		return err
	}

	t.logger.InfoContext(ctx, "transient error, retrying",
		slog.Int("attempt", attempt+1),
		slog.Any("error", err),
		slog.Any("backoff", t.Options.Backoff.duration(attempt)),
	)

	if waitErr := t.Options.Backoff.wait(ctx, attempt); waitErr != nil {
		return nil //nolint:nilerr // context cancelled during backoff, next runLoop iteration will handle it
	}

	return nil
}

// runLoop runs the ticker loop (interval > 0) or a single invocation (one-shot).
// The second return value (hadProgress) is true when at least one invocation
// of the work function succeeded before the loop returned an error.
func (t *Task) runLoop(ctx context.Context) (error, bool) {
	if t.Options.Interval <= 0 {
		return t.runOnce(ctx), false
	}

	// Interval mode.
	hadProgress := false

	if !t.Options.SkipInitialRun {
		if err := t.runOnce(ctx); err != nil {
			return err, false
		}

		hadProgress = true
	}

	ticker := time.NewTicker(t.Options.Interval)
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
	if t.Options.Timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, t.Options.Timeout)
		defer cancel()

		t.logger.DebugContext(ctx, "timeout applied", slog.Any("timeout", t.Options.Timeout))
	}

	return t.Work(ctx)
}
