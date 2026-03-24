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

// ShutdownFunc is called during graceful shutdown.
type ShutdownFunc func(ctx context.Context) error

// Option configures a Task. Use With* functions to create options.
type Option func(*taskConfig)

type taskConfig struct {
	timeout  time.Duration
	delay    time.Duration
	shutdown ShutdownFunc
	logger   *slog.Logger
}

// WithTimeout sets a per-invocation timeout for the work function.
// Each call to work gets its own context with this deadline.
func WithTimeout(d time.Duration) Option {
	return func(c *taskConfig) {
		c.timeout = d
	}
}

// WithDelay delays the first execution by the given duration.
// For interval tasks: first tick fires after delay, then every interval.
// For one-shot tasks: execution starts after delay.
// Delay is independent of interval.
func WithDelay(d time.Duration) Option {
	return func(c *taskConfig) {
		c.delay = d
	}
}

// WithShutdown registers a graceful shutdown hook for the task.
// The hook is called by Runner after all task goroutines have stopped.
func WithShutdown(fn ShutdownFunc) Option {
	return func(c *taskConfig) {
		c.shutdown = fn
	}
}

// WithLogger sets a custom logger for the task. Defaults to slog.Default().
func WithLogger(l *slog.Logger) Option {
	return func(c *taskConfig) {
		c.logger = l
	}
}
