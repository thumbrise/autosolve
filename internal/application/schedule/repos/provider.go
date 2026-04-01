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

// Package repos is the repository-scoped job provider for the scheduler.
//
// Provider is the router. It registers domain tasks as jobs, assigns names,
// intervals, and phases. Each job closure is a handler — a thin adapter that
// knows how to call its domain task with the right partition context.
// Domain tasks don't conform to any provider interface — handlers adapt.
//
// Like Laravel's Route::get('/users', fn() => ...) — handlers can be inline
// closures when the adaptation is trivial. When it grows complex, extract
// a handler struct.
package repos

import (
	"context"
	"time"

	"github.com/google/wire"

	"github.com/thumbrise/autosolve/internal/application/schedule/sdsl"
	"github.com/thumbrise/autosolve/internal/config"
	"github.com/thumbrise/autosolve/internal/domain/contracts"
	"github.com/thumbrise/autosolve/internal/domain/tasks/repository"
)

// Bindings provides Wire bindings for the repository job provider.
var Bindings = wire.NewSet(
	NewProvider,

	repository.NewValidator,
	repository.NewIssuePoller,
	repository.NewOutboxRelay,
)

// Worker is a task scoped to a repository partition.
// Domain implements Run — nothing else. Name, interval, phase — provider's concern.
type Worker interface {
	Run(ctx context.Context, partition repository.Partition) error
}

// Provider multiplies domain workers by configured repositories.
// It is the router — it names tasks, assigns intervals, groups by phase.
// Domain workers just implement Run(ctx, partition).
type Provider struct {
	cfg   *config.Github
	store contracts.RepositoryStore

	validator *repository.Validator
	poller    *repository.IssuePoller
	relay     *repository.OutboxRelay
}

// NewProvider creates a repository job provider.
func NewProvider(
	cfg *config.Github,
	store contracts.RepositoryStore,
	validator *repository.Validator,
	poller *repository.IssuePoller,
	relay *repository.OutboxRelay,
) *Provider {
	return &Provider{cfg: cfg, store: store, validator: validator, poller: poller, relay: relay}
}

// workerRoute defines a recurring task with lazy repoID resolution.
type workerRoute struct {
	name     string
	interval time.Duration
	worker   Worker
}

// Jobs returns ready-to-schedule jobs for all configured repositories.
// The provider is the router — it knows names, intervals, and phases.
func (p *Provider) Jobs() []sdsl.Job {
	workers := []workerRoute{
		{"issue-poller", p.cfg.Issues.ParseInterval, p.poller},
		{"outbox-relay", 5 * time.Second, p.relay},
	}

	repos := p.cfg.Repositories
	jobs := make([]sdsl.Job, 0, (1+len(workers))*len(repos)) // 1 setup + N workers per repo

	// Setup handlers — inline closures, trivial adaptation (no repoID needed).
	for _, repo := range repos {
		r := repo
		jobs = append(jobs, sdsl.SetupJob(
			"repository-validator:"+r.Owner+"/"+r.Name,
			func(ctx context.Context) error {
				return p.validator.Run(ctx, repository.Partition{Owner: r.Owner, Name: r.Name})
			},
		))
	}

	// Worker handlers — closures with lazy repoID resolution via bindWithRepoID.
	for _, repo := range repos {
		r := repo

		for _, rt := range workers {
			jobs = append(jobs, sdsl.WorkerJob(
				rt.name+":"+r.Owner+"/"+r.Name,
				rt.interval,
				p.bindWithRepoID(rt.worker.Run, r),
			))
		}
	}

	return jobs
}

// bindWithRepoID creates a handler closure that resolves repoID lazily.
// The repository ID is resolved on first call — workers depend on setup phase
// having created the DB row. This is the non-trivial handler adaptation.
func (p *Provider) bindWithRepoID(
	work func(context.Context, repository.Partition) error,
	repo config.Repository,
) func(context.Context) error {
	var repoID int64

	return func(ctx context.Context) error {
		if repoID == 0 {
			id, err := p.store.GetIDByOwnerAndName(ctx, repo.Owner, repo.Name)
			if err != nil {
				return err
			}

			repoID = id
		}

		return work(ctx, repository.Partition{
			Owner:        repo.Owner,
			Name:         repo.Name,
			RepositoryID: repoID,
		})
	}
}
