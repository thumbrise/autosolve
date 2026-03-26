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

package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/thumbrise/autosolve/pkg/longrun"
)

// Scheduler orchestrates execution in two phases:
//  1. Preflights — one-shot tasks, all must pass before workers start.
//  2. Workers — long-running interval tasks.
//
// Scheduler is generic — it doesn't know about repositories, GitHub, or issues.
// It only knows phases and task units provided by Planner.
type Scheduler struct {
	planner *Planner
	logger  *slog.Logger
}

func NewScheduler(planner *Planner, logger *slog.Logger) *Scheduler {
	return &Scheduler{planner: planner, logger: logger}
}

func (s *Scheduler) Run(ctx context.Context) error {
	s.logger.InfoContext(ctx, "scheduler: running preflights")

	if err := s.runPreflights(ctx); err != nil {
		return fmt.Errorf("preflights failed: %w", err)
	}

	s.logger.InfoContext(ctx, "scheduler: preflights done, starting workers")

	return s.runWorkers(ctx)
}

func (s *Scheduler) runPreflights(ctx context.Context) error {
	runner := longrun.NewRunner(longrun.RunnerOptions{Logger: s.logger})

	for _, u := range s.planner.Preflights() {
		name := fmt.Sprintf("preflight:%s:%s/%s", u.Resource, u.Repo.Owner, u.Repo.Name)
		runner.Add(longrun.NewOneShotTask(name, u.Work, u.Rules))
	}

	return runner.Wait(ctx)
}

func (s *Scheduler) runWorkers(ctx context.Context) error {
	runner := longrun.NewRunner(longrun.RunnerOptions{Logger: s.logger})

	for _, u := range s.planner.Workers() {
		name := fmt.Sprintf("worker:%s:%s/%s", u.Resource, u.Repo.Owner, u.Repo.Name)
		runner.Add(longrun.NewIntervalTask(name, u.Interval, u.Work, u.Rules))
	}

	return runner.Wait(ctx)
}
