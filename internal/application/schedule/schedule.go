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

package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/thumbrise/autosolve/internal/application/schedule/sdsl"
	"github.com/thumbrise/autosolve/pkg/resilience"
)

// Scheduler is the lifecycle engine. It knows its phases, their order,
// and their retry strategies. Scheduler owns this knowledge — it's not
// configurable, it's the core.
type Scheduler struct {
	plan   *Plan
	client *resilience.Client
	logger *slog.Logger
}

// NewScheduler creates a Scheduler.
func NewScheduler(plan *Plan, client *resilience.Client, logger *slog.Logger) *Scheduler {
	return &Scheduler{plan: plan, client: client, logger: logger}
}

// Run executes the lifecycle: setup first, then work.
func (s *Scheduler) Run(ctx context.Context) error {
	// Setup phase — strict: unregistered errors crash.
	s.logger.InfoContext(ctx, "scheduler: starting setup",
		slog.Int("jobs", len(s.plan.Setup)),
	)

	if err := s.runJobs(ctx, s.plan.Setup, strictRetryOptions()); err != nil {
		return fmt.Errorf("setup: %w", err)
	}

	s.logger.InfoContext(ctx, "scheduler: setup done, starting work",
		slog.Int("jobs", len(s.plan.Work)),
	)

	// Work phase — resilient: catch-all for unregistered errors.
	return s.runJobs(ctx, s.plan.Work, resilientRetryOptions())
}

// runJobs runs all jobs in parallel with the given retry options.
func (s *Scheduler) runJobs(ctx context.Context, jobs []sdsl.Job, opts []resilience.Option) error {
	grp, ctx := errgroup.WithContext(ctx)

	for _, j := range jobs {
		grp.Go(func() error {
			if j.Interval <= 0 {
				return s.runOnce(ctx, j, opts)
			}

			return s.runLoop(ctx, j, opts)
		})
	}

	return grp.Wait()
}

// runOnce executes a job once through the resilience client.
func (s *Scheduler) runOnce(ctx context.Context, j sdsl.Job, opts []resilience.Option) error {
	s.logger.InfoContext(ctx, "job starting", slog.String("job", j.Name))

	err := s.client.Call(j.Work).With(opts...).Do(ctx)
	if err != nil && ctx.Err() == nil {
		return err
	}

	return nil
}

// runLoop executes a job on a ticker through the resilience client.
// Runs immediately, then on every tick.
func (s *Scheduler) runLoop(ctx context.Context, j sdsl.Job, opts []resilience.Option) error {
	s.logger.InfoContext(ctx, "job starting",
		slog.String("job", j.Name),
		slog.Any("interval", j.Interval),
	)

	ticker := time.NewTicker(j.Interval)
	defer ticker.Stop()

	for {
		err := s.client.Call(j.Work).With(opts...).Do(ctx)
		if err != nil && ctx.Err() == nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
