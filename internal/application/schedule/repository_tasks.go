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

	"github.com/thumbrise/autosolve/internal/config"
	"github.com/thumbrise/autosolve/internal/domain"
	"github.com/thumbrise/autosolve/internal/domain/spec"
	"github.com/thumbrise/autosolve/internal/domain/spec/repository"
)

// RepositoryTasks multiplies TaskSpecs by configured repositories.
// Each spec × each repo = one Task. Preflights get partition without repoID,
// work-phase tasks get partition with lazy-resolved repoID.
type RepositoryTasks struct {
	cfg   *config.Github
	store domain.RepositoryStore
}

// NewRepositoryTasks creates a RepositoryTasks.
func NewRepositoryTasks(cfg *config.Github, store domain.RepositoryStore) *RepositoryTasks {
	return &RepositoryTasks{cfg: cfg, store: store}
}

// repoTask is an application-level wrapper around domain TaskSpec.
// Carries phase metadata that domain doesn't know about.
type repoTask struct {
	spec  repository.TaskSpec
	phase spec.Phase
}

// Preflight marks a repository TaskSpec as a preflight task.
// Preflights run before all other tasks. The partition may have incomplete data
// (e.g. Partition.RepositoryID may be zero).
//
// Usage in registry:
//
//	repos.Pack(
//	    schedule.Preflight(repoValidator.TaskSpec()),
//	    issuePoller.TaskSpec(),
//	)
func Preflight(s repository.TaskSpec) repoTask {
	return repoTask{spec: s, phase: spec.PhasePreflight}
}

// resolveEntry extracts TaskSpec and Phase from a Pack entry.
// Accepts repository.TaskSpec (default PhaseWork) or Preflight()-wrapped repoTask.
// Panics on unknown type or zero Interval.
func resolveEntry(entry any) (repository.TaskSpec, spec.Phase) {
	var s repository.TaskSpec

	var phase spec.Phase

	switch v := entry.(type) {
	case repository.TaskSpec:
		s = v
		phase = spec.PhaseWork
	case repoTask:
		s = v.spec
		phase = v.phase
	default:
		panic(fmt.Sprintf("schedule: Pack accepts repository.TaskSpec or Preflight(), got %T", entry))
	}

	if s.Interval == 0 {
		panic(fmt.Sprintf("schedule: spec %q has zero Interval — use spec.OneShot", s.Resource))
	}

	return s, phase
}

// Pack multiplies specs by configured repositories and returns ready-to-schedule Tasks.
//
// Accepts repository.TaskSpec (default PhaseWork) or Preflight()-wrapped specs.
// Panics on zero Interval (must use spec.OneShot) or unknown entry type.
func (rt *RepositoryTasks) Pack(entries ...any) []spec.Task {
	tasks := make([]spec.Task, 0, len(entries)*len(rt.cfg.Repositories))

	for _, entry := range entries {
		s, phase := resolveEntry(entry)

		for _, repo := range rt.cfg.Repositories {
			r := repo

			var repoID int64

			prefix := "worker"
			if phase == spec.PhasePreflight {
				prefix = "preflight"
			}

			work := s.Work
			tasks = append(tasks, spec.Task{
				Name:     fmt.Sprintf("%s:%s:%s/%s", prefix, s.Resource, r.Owner, r.Name),
				Interval: s.Interval,
				Phase:    phase,
				Work: func(ctx context.Context) error {
					if phase == spec.PhaseWork && repoID == 0 {
						id, err := rt.store.GetIDByOwnerAndName(ctx, r.Owner, r.Name)
						if err != nil {
							return err
						}

						repoID = id
					}

					return work(ctx, repository.Partition{
						Owner:        r.Owner,
						Name:         r.Name,
						RepositoryID: repoID,
					})
				},
			})
		}
	}

	return tasks
}
