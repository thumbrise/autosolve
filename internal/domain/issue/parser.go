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

package issue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/go-github/v84/github"

	"github.com/thumbrise/autosolve/internal/config"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/model"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/repositories"
	githubinfra "github.com/thumbrise/autosolve/internal/infrastructure/github"
)

// Sentinel errors for classifying parser failures.
// Callers can use these with errors.Is to decide retry strategy.
var (
	ErrFetchIssues    = errors.New("fetch issues")
	ErrStoreIssues    = errors.New("store issues")
	ErrReadLastUpdate = errors.New("read last update")
)

type Parser struct {
	githubClient    *githubinfra.Client
	logger          *slog.Logger
	issueRepository *repositories.IssueRepository
	cfg             *config.Github
}

func NewParser(cfg *config.Github, githubClient *githubinfra.Client, issueRepository *repositories.IssueRepository, logger *slog.Logger) *Parser {
	return &Parser{cfg: cfg, githubClient: githubClient, issueRepository: issueRepository, logger: logger}
}

func (p *Parser) Run(ctx context.Context) error {
	p.logger.DebugContext(ctx, "starting request to list issues")

	lastUpdate, err := p.lastUpdate(ctx)
	if err != nil {
		// SQLLite 1 connection pool. Always permanent
		return fmt.Errorf("%w: %w", ErrReadLastUpdate, err)
	}

	issues, _, err := p.githubClient.GetMostUpdatedIssues(ctx, 50, lastUpdate)
	if err != nil {
		if p.adaptRateLimit(ctx, err) {
			// rate limit was caught
			// just waiting next tick
			return nil
		}

		return fmt.Errorf("%w: list by repo: %w", ErrFetchIssues, err)
	}

	if len(issues) == 0 {
		// noop for now
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

func (p *Parser) store(ctx context.Context, issues []*github.Issue) error {
	models := make([]*model.Issue, 0, len(issues))
	for _, issue := range issues {
		models = append(models, p.mapIssueToModel(issue))
	}

	return p.issueRepository.UpsertMany(ctx, models)
}

func (p *Parser) mapIssueToModel(issue *github.Issue) *model.Issue {
	state := model.IssueStateOpen
	if issue.GetState() == "closed" {
		state = model.IssueStateClosed
	}

	now := time.Now()

	result := &model.Issue{
		GithubID:        issue.GetID(),
		Number:          int64(issue.GetNumber()),
		Title:           issue.GetTitle(),
		Body:            issue.GetBody(),
		State:           state,
		IsPullRequest:   issue.IsPullRequest(),
		GithubCreatedAt: issue.GetCreatedAt().Time,
		GithubUpdatedAt: issue.GetUpdatedAt().Time,
		SyncedAt:        now,
	}
	if issue.PullRequestLinks != nil {
		result.PRUrl = issue.PullRequestLinks.URL
		result.PRHtmlUrl = issue.PullRequestLinks.HTMLURL
		result.PRDiffUrl = issue.PullRequestLinks.DiffURL
		result.PRPatchUrl = issue.PullRequestLinks.PatchURL
	}
	//nolint:godox // milestone temp
	// TODO: labels and assignees via M:N — separate step
	return result
}

func (p *Parser) lastUpdate(ctx context.Context) (time.Time, error) {
	res, err := p.issueRepository.GetLastUpdateTime(ctx)
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
func (p *Parser) adaptRateLimit(ctx context.Context, err error) bool {
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
func (p *Parser) adaptPollingInterval(_ context.Context) {
	//nolint:godox // noop: will be implemented when adaptive polling is prioritized in https://github.com/thumbrise/autosolve/issues/53
	// TODO: implement exponential backoff on empty responses, reset to base interval when data appears.
}
