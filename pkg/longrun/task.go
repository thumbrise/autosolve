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

	"github.com/thumbrise/autosolve/pkg/resilience"
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
	resOpts  []resilience.Option
	logger   *slog.Logger
}

// NewOneShotTask creates a task that executes once.
// Without resilience options — any error is fatal.
// With options — transient errors are retried per their configuration.
//
// resOpts are [resilience.Option] middleware applied to each invocation.
// Use [retry.On] to add retry rules, or any other resilience pattern.
// Task-level options are applied first; Runner-level options (baseline) are
// appended at [Runner.Add] time.
//
// taskOpts configure task lifecycle (timeout, delay, shutdown, logger).
//
// Panics if work is nil.
func NewOneShotTask(name string, work WorkFunc, resOpts []resilience.Option, taskOpts ...Option) *Task {
	return newTask(name, 0, work, resOpts, taskOpts)
}

// NewIntervalTask creates a task that runs on a ticker loop.
// Without resilience options — any error kills the task.
// With options — transient errors are retried per their configuration,
// permanent errors (no matching rule) kill the task.
//
// resOpts are [resilience.Option] middleware applied to each invocation.
// Use [retry.On] to add retry rules, or any other resilience pattern.
// Task-level options are applied first; Runner-level options (baseline) are
// appended at [Runner.Add] time.
//
// taskOpts configure task lifecycle (timeout, delay, shutdown, logger).
//
// Panics if work is nil or interval <= 0.
func NewIntervalTask(name string, interval time.Duration, work WorkFunc, resOpts []resilience.Option, taskOpts ...Option) *Task {
	if interval <= 0 {
		panic("longrun.NewIntervalTask: interval must be > 0")
	}

	return newTask(name, interval, work, resOpts, taskOpts)
}

func newTask(name string, interval time.Duration, work WorkFunc, resOpts []resilience.Option, taskOpts []Option) *Task {
	if work == nil {
		panic("longrun: work function must not be nil")
	}

	cfg := &taskConfig{}
	for _, o := range taskOpts {
		o(cfg)
	}

	logger := cfg.logger
	if logger == nil {
		logger = slog.Default()
	}

	logger = logger.With(slog.String("task", name))

	return &Task{
		name:     name,
		work:     work,
		shutdown: cfg.shutdown,
		interval: interval,
		timeout:  cfg.timeout,
		delay:    cfg.delay,
		resOpts:  resOpts,
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
		slog.Int("resilience_opts", len(t.resOpts)),
	)

	err := t.restartLoop(ctx)
	if err != nil {
		t.logger.ErrorContext(ctx, "permanent error", slog.Any("error", err))

		return err
	}

	t.logger.InfoContext(ctx, "stopped")

	return nil
}
