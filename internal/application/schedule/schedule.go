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

	"github.com/thumbrise/autosolve/pkg/longrun"
)

// Scheduler orchestrates execution in two phases:
//  1. Preflights — one-shot tasks, all must pass before workers start.
//  2. Workers — long-running interval tasks (per-repo and global).
//
// Scheduler is generic — it doesn't know about repositories, GitHub, or issues.
// It only knows phases and task units provided by Planner and global workers.
type Scheduler struct {
	planner       *Planner
	globalWorkers []GlobalWorker
	logger        *slog.Logger
}

func NewScheduler(planner *Planner, globalWorkers []GlobalWorker, logger *slog.Logger) *Scheduler {
	return &Scheduler{planner: planner, globalWorkers: globalWorkers, logger: logger}
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
	runner := longrun.NewRunner(longrun.RunnerOptions{
		Logger: s.logger,
		Baseline: longrun.Baseline{
			// Transport errors — aggressive retry, network will recover.
			Node: longrun.Policy{Backoff: longrun.Backoff(2*time.Second, 2*time.Minute)},
			// Service pressure — gentle retry, don't kick them while they're down.
			Service: longrun.Policy{Backoff: longrun.Backoff(5*time.Second, 5*time.Minute)},
			// Degraded: nil — unknown errors crash preflights. Fix your config.
			Classify: s.planner.InfraClassifier(),
		},
	})

	for _, u := range s.planner.Preflights() {
		name := fmt.Sprintf("preflight:%s:%s/%s", u.Resource, u.Repo.Owner, u.Repo.Name)
		runner.Add(longrun.NewOneShotTask(name, u.Work, nil))
	}

	return runner.Wait(ctx)
}

func (s *Scheduler) runWorkers(ctx context.Context) error {
	runner := longrun.NewRunner(longrun.RunnerOptions{
		Logger: s.logger,
		Baseline: longrun.Baseline{
			// Transport errors — aggressive retry, network will recover.
			Node: longrun.Policy{Backoff: longrun.Backoff(2*time.Second, 2*time.Minute)},
			// Service pressure — gentle retry, don't kick them while they're down.
			Service: longrun.Policy{Backoff: longrun.Backoff(5*time.Second, 5*time.Minute)},
			// Unknown errors — don't crash, scream loudly, retry with big backoff.
			Degraded: &longrun.Policy{Backoff: longrun.Backoff(30*time.Second, 5*time.Minute)},
			Classify: s.planner.InfraClassifier(),
		},
	})

	for _, u := range s.planner.Workers() {
		name := fmt.Sprintf("worker:%s:%s/%s", u.Resource, u.Repo.Owner, u.Repo.Name)
		runner.Add(longrun.NewIntervalTask(name, u.Interval, u.Work, nil))
	}

	// Global workers — not multiplied per repository.
	for _, gw := range s.globalWorkers {
		s := gw.TaskSpec()
		name := "worker:" + s.Resource
		runner.Add(longrun.NewIntervalTask(name, s.Interval, s.Work, nil))
	}

	return runner.Wait(ctx)
}
