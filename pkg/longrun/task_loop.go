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
	"go.opentelemetry.io/otel/codes"
)

// restartLoop implements the restart loop with backoff.
//
//	Task.Wait(ctx)
//	  └→ restartLoop (restart loop + backoff)
//	       └→ runLoop (ticker or one-shot)
//	            └→ runOnce (single invocation ± timeout)
func (t *Task) restartLoop(ctx context.Context) error {
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

// runLoop runs the ticker loop (interval > 0) or a single invocation (one-shot).
// The second return value (hadProgress) is true when at least one invocation
// of the work function succeeded before the loop returned an error.
//
// Interval mode always runs work immediately, then on every tick. When a transient
// error triggers a retry via handleFailure, restartLoop re-enters runLoop from
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
