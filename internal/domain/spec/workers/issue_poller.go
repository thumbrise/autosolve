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

	"github.com/thumbrise/autosolve/internal/config"
	"github.com/thumbrise/autosolve/internal/domain/entities"
	"github.com/thumbrise/autosolve/internal/domain/spec"
	"github.com/thumbrise/autosolve/internal/domain/spec/resources"
	"github.com/thumbrise/autosolve/internal/domain/spec/tenants"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/repositories"
	githubinfra "github.com/thumbrise/autosolve/internal/infrastructure/github"
	"github.com/thumbrise/autosolve/pkg/httperr"
)

// Sentinel errors for classifying poller failures.
// Callers can use these with errors.Is to decide retry strategy.
var (
	ErrFetchIssues = errors.New("fetch issues")
	ErrStoreIssues = errors.New("store issues")
	ErrReadCursor  = errors.New("read cursor")
	ErrSaveCursor  = errors.New("save cursor")
)

// IssuePoller fetches and stores issues for a repository.
// Stateless per repository — owner, repo and repositoryID are received
// via RepositoryTenant at each invocation.
// Implements application.Worker via TaskSpec().
type IssuePoller struct {
	githubClient *githubinfra.Client
	logger       *slog.Logger
	issueRepo    *repositories.IssueRepository
	cursorRepo   *repositories.SyncCursorRepository
	cfg          *config.Github
}

func NewIssuePoller(
	cfg *config.Github,
	githubClient *githubinfra.Client,
	issueRepo *repositories.IssueRepository,
	cursorRepo *repositories.SyncCursorRepository,
	logger *slog.Logger,
) *IssuePoller {
	return &IssuePoller{
		cfg:          cfg,
		githubClient: githubClient,
		issueRepo:    issueRepo,
		cursorRepo:   cursorRepo,
		logger:       logger,
	}
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

	cursor, err := p.findCursor(ctx, tenant.RepositoryID)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrReadCursor, err)
	}

	resp, err := p.githubClient.GetMostUpdatedIssues(ctx, p.buildRequest(tenant, cursor))
	if err != nil {
		if p.adaptRateLimit(ctx, err) {
			return nil
		}

		return fmt.Errorf("%w: %w", ErrFetchIssues, err)
	}

	if resp.NotModified {
		p.logger.InfoContext(ctx, "not modified, skipping")

		return nil
	}

	if len(resp.Issues) == 0 {
		p.adaptPollingInterval(ctx)

		p.logger.InfoContext(ctx, "no new issues found")

		return nil
	}

	p.logger.InfoContext(ctx, "fetched", slog.Int("count", len(resp.Issues)))

	if err := p.issueRepo.UpsertMany(ctx, tenant.RepositoryID, resp.Issues); err != nil {
		return fmt.Errorf("%w: %w", ErrStoreIssues, err)
	}

	if err := p.saveCursor(ctx, tenant.RepositoryID, resp.NextCursor); err != nil {
		return fmt.Errorf("%w: %w", ErrSaveCursor, err)
	}

	p.logger.InfoContext(ctx, "stored", slog.Int("count", len(resp.Issues)))

	return nil
}

func (p *IssuePoller) buildRequest(tenant tenants.RepositoryTenant, cursor entities.SyncCursor) githubinfra.Request {
	req := githubinfra.Request{
		Owner:      tenant.Owner,
		Repository: tenant.Name,
		Cursor: githubinfra.Cursor{
			Limit: 50,
			Page:  cursor.NextPage,
			Since: cursor.SinceUpdatedAt,
		},
	}

	// ETag only in steady-state (page 1). During catch-up ETag is invalid for other pages.
	isCatchUp := cursor.NextPage > 1
	if !isCatchUp {
		req.Cursor.ETag = cursor.ETag
	}

	return req
}

func (p *IssuePoller) saveCursor(ctx context.Context, repositoryID int64, next githubinfra.Cursor) error {
	return p.cursorRepo.Save(ctx, entities.SyncCursor{
		RepositoryID:   repositoryID,
		ResourceType:   string(resources.Issue),
		SinceUpdatedAt: next.Since,
		NextPage:       next.Page,
		ETag:           next.ETag,
	})
}

// findCursor loads the sync cursor from the database.
// Returns a zero-value SyncCursor when no cursor exists yet (first run).
func (p *IssuePoller) findCursor(ctx context.Context, repositoryID int64) (entities.SyncCursor, error) {
	cursor, err := p.cursorRepo.Find(ctx, repositoryID, resources.Issue)
	if err != nil {
		if dal.IsNotFound(err) {
			p.logger.DebugContext(ctx, "cursor not found, starting from scratch",
				slog.Int64("repositoryId", repositoryID),
			)

			return entities.SyncCursor{}, nil
		}

		return entities.SyncCursor{}, err
	}

	p.logger.DebugContext(ctx, "cursor found",
		slog.Int64("repositoryId", repositoryID),
		slog.Time("since", cursor.SinceUpdatedAt),
		slog.Int("nextPage", cursor.NextPage),
		slog.String("etag", cursor.ETag),
	)

	return cursor, nil
}

// adaptRateLimit pauses execution until the rate limit resets.
// Uses our RateLimitError (not go-github types) to extract RetryAfter.
//
// If err contains *githubinfra.RateLimitError with positive RetryAfter — sleeps and returns true.
// If RetryAfter is zero — returns false, letting longrun handle retry via exponential backoff.
// For all other errors — returns false.
func (p *IssuePoller) adaptRateLimit(ctx context.Context, err error) bool {
	var rlErr *githubinfra.RateLimitError
	if !errors.As(err, &rlErr) {
		return false
	}

	if rlErr.RetryAfter <= 0 {
		p.logger.WarnContext(ctx, "rate limit without RetryAfter, falling back to longrun backoff")

		return false
	}

	p.logger.WarnContext(ctx, "rate limit hit, pausing",
		slog.Duration("wait", rlErr.RetryAfter),
	)

	timer := time.NewTimer(rlErr.RetryAfter)
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
