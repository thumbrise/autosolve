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

	"github.com/thumbrise/autosolve/pkg/resilience"
)

// restartLoop implements the execution loop.
//
// One-shot tasks: resilience.Do handles retry via Options, returns on
// success or permanent error.
//
// Interval tasks: run immediately, then on every tick. Each invocation
// is wrapped with resilience.Do — transient errors retry with backoff,
// permanent errors kill the task.
//
//	Task.Wait(ctx)
//	  └→ restartLoop
//	       └→ resilience.Do(ctx, runOnce, resOpts...)
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

	if t.interval <= 0 {
		return t.suppressCtxErr(ctx, t.doOnce(ctx))
	}

	return t.suppressCtxErr(ctx, t.doInterval(ctx))
}

// suppressCtxErr returns nil when the error is caused by context cancellation.
// Context cancellation is not a task error — it's an external signal.
func (t *Task) suppressCtxErr(ctx context.Context, err error) error {
	if err != nil && ctx.Err() != nil {
		return nil //nolint:nilerr // special case, function name reflect it
	}

	return err
}

// doOnce executes the work function once with resilience options.
// Used for one-shot tasks.
func (t *Task) doOnce(ctx context.Context) error {
	return resilience.Do(ctx, t.runOnce, t.resOpts...)
}

// doInterval runs the ticker loop. Each tick is wrapped with resilience.Do —
// transient errors retry with backoff, permanent errors kill the task.
// On success, the loop waits for the next tick.
func (t *Task) doInterval(ctx context.Context) error {
	// Run immediately before starting the ticker.
	if err := resilience.Do(ctx, t.runOnce, t.resOpts...); err != nil {
		return err
	}

	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := resilience.Do(ctx, t.runOnce, t.resOpts...); err != nil {
				return err
			}
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
