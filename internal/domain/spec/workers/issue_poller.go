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

package workers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/go-github/v84/github"

	"github.com/thumbrise/autosolve/internal/config"
	"github.com/thumbrise/autosolve/internal/domain/entities"
	"github.com/thumbrise/autosolve/internal/domain/spec"
	"github.com/thumbrise/autosolve/internal/domain/spec/tenants"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/repositories"
	githubinfra "github.com/thumbrise/autosolve/internal/infrastructure/github"
	"github.com/thumbrise/autosolve/pkg/httperr"
)

// Sentinel errors for classifying parser failures.
// Callers can use these with errors.Is to decide retry strategy.
var (
	ErrFetchIssues    = errors.New("fetch issues")
	ErrStoreIssues    = errors.New("store issues")
	ErrReadLastUpdate = errors.New("read last update")
)

// IssuePoller fetches and stores issues for a repository.
// Stateless per repository — owner, repo and repositoryID are received
// via RepositoryTenant at each invocation.
// Implements application.Worker via TaskSpec().
type IssuePoller struct {
	githubClient    *githubinfra.Client
	logger          *slog.Logger
	issueRepository *repositories.IssueRepository
	cfg             *config.Github
}

func NewIssuePoller(cfg *config.Github, githubClient *githubinfra.Client, issueRepository *repositories.IssueRepository, logger *slog.Logger) *IssuePoller {
	return &IssuePoller{cfg: cfg, githubClient: githubClient, issueRepository: issueRepository, logger: logger}
}

// TaskSpec returns a WorkerSpec for polling issues.
func (p *IssuePoller) TaskSpec() spec.WorkerSpec {
	return spec.WorkerSpec{
		Resource:   "issue-poller",
		Interval:   p.cfg.Issues.ParseInterval,
		Transients: httperr.TransientErrors(),
		Work:       p.Run,
	}
}

func (p *IssuePoller) Run(ctx context.Context, tenant tenants.RepositoryTenant) error {
	p.logger.DebugContext(ctx, "starting request to list issues",
		slog.String("owner", tenant.Owner),
		slog.String("name", tenant.Name),
	)

	lastUpdate, err := p.lastUpdate(ctx, tenant.RepoID)
	if err != nil {
		// SQLLite 1 connection pool. Always permanent
		return fmt.Errorf("%w: %w", ErrReadLastUpdate, err)
	}

	req := githubinfra.Request{
		Owner:      tenant.Owner,
		Repository: tenant.Name,
		Cursor: githubinfra.Cursor{
			Limit: 50,
			Since: lastUpdate,
			ETag:  "",
		},
	}

	issues, _, err := p.githubClient.GetMostUpdatedIssues(ctx, req)
	if err != nil {
		if p.adaptRateLimit(ctx, err) {
			// rate limit was caught
			// just waiting next tick
			return nil
		}

		return fmt.Errorf("%w: list by repo: %w", ErrFetchIssues, err)
	}

	if len(issues) == 0 {
		p.adaptPollingInterval(ctx)

		p.logger.InfoContext(ctx, "no new issues found")

		return nil
	}

	p.logger.InfoContext(ctx, "fetched", slog.Int("count", len(issues)))

	err = p.store(ctx, issues)
	if err != nil {
		// SQLite 1 connection pool. Always permanent
		return fmt.Errorf("%w: %w", ErrStoreIssues, err)
	}

	p.logger.InfoContext(ctx, "stored", slog.Int("count", len(issues)))

	return nil
}

func (p *IssuePoller) store(ctx context.Context, issues []*entities.Issue) error {
	return p.issueRepository.UpsertMany(ctx, issues)
}

func (p *IssuePoller) lastUpdate(ctx context.Context, repositoryID int64) (time.Time, error) {
	res, err := p.issueRepository.GetLastUpdateTime(ctx, repositoryID)
	if err != nil {
		if dal.IsNotFound(err) {
			return time.Time{}, nil
		}

		return time.Time{}, err
	}

	return res.Add(1 * time.Second), nil
}

// adaptRateLimit pauses execution until the rate limit resets.
// Called when the GitHub API returns a rate limit error.
// The original error is available in the chain via errors.As
// for accessing reset time and remaining quota.
//
// If err is *github.RateLimitError or *github.AbuseRateLimitError (with RetryAfter set) then returns true after sleeping.
// If err is *github.AbuseRateLimitError without RetryAfter, returns false to let the caller handle retry via exponential backoff.
// For all other errors, returns false and you should handle original error.
func (p *IssuePoller) adaptRateLimit(ctx context.Context, err error) bool {
	var (
		wait     time.Duration
		rlErr    *github.RateLimitError
		abuseErr *github.AbuseRateLimitError
	)

	switch {
	case errors.As(err, &rlErr):
		wait = time.Until(rlErr.Rate.Reset.Time)
	case errors.As(err, &abuseErr) && abuseErr.RetryAfter != nil:
		wait = *abuseErr.RetryAfter
	case errors.As(err, &abuseErr):
		p.logger.WarnContext(ctx, "abuse rate limit hit without RetryAfter header, falling back to exponential backoff")

		return false
	default:
		return false
	}

	p.logger.WarnContext(ctx, "rate limit hit, pausing",
		slog.Duration("wait", wait),
	)

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}

	return true
}

// adaptPollingInterval adjusts the polling frequency based on data availability.
// Called when a successful fetch returns zero issues, indicating a quiet period.
// Reduces GitHub API load by increasing the interval between polls.
//
// noop for now
func (p *IssuePoller) adaptPollingInterval(_ context.Context) {
	//nolint:godox // noop: will be implemented when adaptive polling is prioritized in https://github.com/thumbrise/autosolve/issues/53
	// TODO: implement exponential backoff on empty responses, reset to base interval when data appears.
}
