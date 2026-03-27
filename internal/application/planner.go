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
	"time"

	"github.com/thumbrise/autosolve/internal/config"
	"github.com/thumbrise/autosolve/internal/domain/spec/tenants"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/repositories"
	"github.com/thumbrise/autosolve/pkg/longrun"
)

// PreflightUnit is a ready-to-schedule one-shot task produced by Planner.
type PreflightUnit struct {
	Resource string
	Repo     config.Repository
	Rules    []longrun.TransientRule
	Work     longrun.WorkFunc
}

// WorkerUnit is a ready-to-schedule interval task produced by Planner.
type WorkerUnit struct {
	Resource string
	Repo     config.Repository
	Interval time.Duration
	Rules    []longrun.TransientRule
	Work     longrun.WorkFunc
}

// Planner builds per-repository task units from domain specs.
// It owns the per-repo concept: each repository in config produces
// a set of preflight and worker units.
type Planner struct {
	cfg        *config.Github
	preflights []Preflight
	workers    []Worker
	repoRepo   *repositories.RepositoryRepository
}

func NewPlanner(
	cfg *config.Github,
	preflights []Preflight,
	workers []Worker,
	repoRepo *repositories.RepositoryRepository,
) *Planner {
	return &Planner{cfg: cfg, preflights: preflights, workers: workers, repoRepo: repoRepo}
}

// Preflights returns one-shot units for all repositories × all preflight specs.
func (p *Planner) Preflights() []PreflightUnit {
	units := make([]PreflightUnit, 0, len(p.preflights)*len(p.cfg.Repositories))

	for _, pf := range p.preflights {
		s := pf.TaskSpec()

		for _, repo := range p.cfg.Repositories {
			r := repo

			units = append(units, PreflightUnit{
				Resource: s.Resource,
				Repo:     r,
				Rules:    p.buildRules(s.Transients),
				Work: func(ctx context.Context) error {
					return s.Work(ctx, tenants.RepositoryTenant{Owner: r.Owner, Name: r.Name})
				},
			})
		}
	}

	return units
}

// Workers returns interval units for all repositories × all worker specs.
// Each unit caches the repository ID on first invocation to avoid repeated lookups.
func (p *Planner) Workers() []WorkerUnit {
	units := make([]WorkerUnit, 0, len(p.workers)*len(p.cfg.Repositories))

	for _, w := range p.workers {
		s := w.TaskSpec()

		for _, repo := range p.cfg.Repositories {
			r := repo

			var repoID int64

			units = append(units, WorkerUnit{
				Resource: s.Resource,
				Repo:     r,
				Interval: s.Interval,
				Rules:    p.buildRules(s.Transients),
				Work: func(ctx context.Context) error {
					if repoID == 0 {
						id, err := p.repoRepo.GetIDByOwnerAndName(ctx, r.Owner, r.Name)
						if err != nil {
							return err
						}

						repoID = id
					}

					return s.Work(ctx, tenants.RepositoryTenant{Owner: r.Owner, Name: r.Name, RepositoryID: repoID})
				},
			})
		}
	}

	return units
}

const defaultMaxRetries = 5

// buildRules converts domain transient errors into longrun retry rules.
// Retry configuration is centralized here — domain only declares which errors are transient.
func (p *Planner) buildRules(transients []error) []longrun.TransientRule {
	if len(transients) == 0 {
		return nil
	}

	return longrun.TransientGroup(defaultMaxRetries, longrun.DefaultBackoff(), transients...)
}
