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

	"golang.org/x/sync/errgroup"
)

// RunnerOptions configures a Runner.
type RunnerOptions struct {
	ShutdownTimeout time.Duration // default 30s
	Logger          *slog.Logger  // nil = slog.Default()
}

// Runner orchestrates N tasks.  When any task returns a permanent error the
// runner cancels all remaining tasks and performs graceful shutdown.
type Runner struct {
	tasks  []*Task
	opts   RunnerOptions
	logger *slog.Logger
}

// NewRunner creates a Runner with the given options.
func NewRunner(opts RunnerOptions) *Runner {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	timeout := opts.ShutdownTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	opts.ShutdownTimeout = timeout

	return &Runner{
		opts:   opts,
		logger: logger,
	}
}

// Add registers a task for concurrent execution.
func (r *Runner) Add(task *Task) {
	r.tasks = append(r.tasks, task)

	r.logger.Debug("runner: task added",
		slog.String("task", task.Name),
	)
}

// Wait starts all tasks concurrently and blocks until they all finish.
// When any task returns an error, all other tasks are cancelled via ctx.
// After cancellation, Shutdown is called on every task that has one.
// The ctx passed in controls the lifetime — the runner does NOT listen for
// OS signals; use signal.NotifyContext in the caller.
func (r *Runner) Wait(ctx context.Context) error {
	r.logger.InfoContext(ctx, "runner starting",
		slog.Int("tasks", len(r.tasks)),
	)

	grp, ctxGrp := errgroup.WithContext(ctx)

	for _, task := range r.tasks {
		grp.Go(func() error {
			return task.Wait(ctxGrp)
		})
	}

	r.logger.InfoContext(ctx, "runner waiting for tasks")

	err := grp.Wait()

	// All task goroutines have finished — safe to run shutdown hooks.
	r.logger.InfoContext(ctx, "all tasks stopped, running shutdown hooks")

	ctxShutdown, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), r.opts.ShutdownTimeout)
	defer shutdownCancel()

	r.shutdownTasks(ctxShutdown)

	if err != nil && !errors.Is(err, context.Canceled) {
		r.logger.ErrorContext(ctx, "runner error", slog.Any("error", err))

		return err
	}

	r.logger.InfoContext(ctx, "runner finished")

	return nil
}

func (r *Runner) shutdownTasks(ctx context.Context) {
	for _, task := range r.tasks {
		if task.Shutdown == nil {
			continue
		}

		logger := r.logger.With(slog.String("task", task.Name))

		err := task.Shutdown(ctx)
		if err != nil {
			logger.ErrorContext(ctx, "failed to shutdown task",
				slog.Any("error", err),
			)

			continue
		}

		logger.InfoContext(ctx, "successfully shutdown")
	}
}
