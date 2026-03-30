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
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/thumbrise/autosolve/internal/contracts/apierr"
	"github.com/thumbrise/autosolve/internal/domain/spec"
	"github.com/thumbrise/autosolve/pkg/longrun"
)

// Scheduler orchestrates execution in two phases:
//  1. Preflights — one-shot tasks, all must pass before workers start.
//  2. Workers — long-running interval tasks.
//
// Scheduler trusts Planner — it receives a flat list of Units and knows
// nothing about repositories, partitions, or scopes.
type Scheduler struct {
	planner *Planner
	logger  *slog.Logger
}

// NewScheduler creates a Scheduler.
func NewScheduler(planner *Planner, logger *slog.Logger) *Scheduler {
	return &Scheduler{planner: planner, logger: logger}
}

// Run executes tasks in lifecycle order: preflights first, then workers.
func (s *Scheduler) Run(ctx context.Context) error {
	preflights := s.planner.Preflights()
	workers := s.planner.Workers()

	s.logger.InfoContext(ctx, "scheduler: running preflights", slog.Int("count", len(preflights)))

	if err := s.runPreflights(ctx, preflights); err != nil {
		return fmt.Errorf("preflights failed: %w", err)
	}

	s.logger.InfoContext(ctx, "scheduler: preflights done, starting workers", slog.Int("count", len(workers)))

	return s.runWorkers(ctx, workers)
}

func (s *Scheduler) runPreflights(ctx context.Context, tasks []spec.Task) error {
	runner := longrun.NewRunner(longrun.RunnerOptions{
		Logger: s.logger,
		// Default: nil — unknown errors crash preflights. Fix your config.
		Baseline: longrun.NewBaseline(
			longrun.Policy{Backoff: longrun.Backoff(2*time.Second, 2*time.Minute)}, // Node — aggressive retry
			longrun.Policy{Backoff: longrun.Backoff(5*time.Second, 5*time.Minute)}, // Service — gentle retry
			infraClassifier(),
		),
	})

	for _, t := range tasks {
		runner.Add(longrun.NewOneShotTask(t.Name, t.Work, nil))
	}

	return runner.Wait(ctx)
}

func (s *Scheduler) runWorkers(ctx context.Context, tasks []spec.Task) error {
	runner := longrun.NewRunner(longrun.RunnerOptions{
		Logger: s.logger,
		// Unknown errors — don't crash, scream loudly, retry with big backoff.
		Baseline: longrun.NewBaselineDegraded(
			longrun.Policy{Backoff: longrun.Backoff(2*time.Second, 2*time.Minute)},  // Node — aggressive retry
			longrun.Policy{Backoff: longrun.Backoff(5*time.Second, 5*time.Minute)},  // Service — gentle retry
			longrun.Policy{Backoff: longrun.Backoff(30*time.Second, 5*time.Minute)}, // Default — degraded
			infraClassifier(),
		),
	})

	for _, t := range tasks {
		runner.Add(longrun.NewIntervalTask(t.Name, t.Interval, t.Work, nil))
	}

	return runner.Wait(ctx)
}

// infraClassifier returns a ClassifierFunc that checks apierr interfaces
// on errors returned by infrastructure clients.
//
// Classification:
//   - apierr.WaitHinted with positive WaitDuration → Service + explicit wait
//   - apierr.ServicePressure → Service
//   - apierr.Retryable → Service
//   - unknown → nil (let baseline handle as Unknown/Degraded)
func infraClassifier() longrun.ClassifierFunc {
	return func(err error) *longrun.ErrorClass {
		var wh apierr.WaitHinted
		if errors.As(err, &wh) && wh.WaitDuration() > 0 {
			return &longrun.ErrorClass{
				Category:     longrun.CategoryService,
				WaitDuration: wh.WaitDuration(),
			}
		}

		var sp apierr.ServicePressure
		if errors.As(err, &sp) && sp.ServicePressure() {
			return &longrun.ErrorClass{Category: longrun.CategoryService}
		}

		var rt apierr.Retryable
		if errors.As(err, &rt) && rt.Retryable() {
			return &longrun.ErrorClass{Category: longrun.CategoryService}
		}

		return nil
	}
}
