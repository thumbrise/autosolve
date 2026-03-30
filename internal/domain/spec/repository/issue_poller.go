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

package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/thumbrise/autosolve/internal/config"
	"github.com/thumbrise/autosolve/internal/domain/entities"
	githubinfra "github.com/thumbrise/autosolve/internal/infrastructure/github"
)

// IssueSyncRepo is the interface the poller depends on — aggregate root for issue sync.
type IssueSyncRepo interface {
	Save(ctx context.Context, repositoryID int64, issues []*entities.Issue, cursor entities.Cursor) error
	Cursor(ctx context.Context, repositoryID int64) (entities.Cursor, error)
}

var (
	ErrFetchIssues = errors.New("fetch issues")
	ErrReadCursor  = errors.New("read cursor")
	ErrSave        = errors.New("save sync")
)

// IssuePoller fetches issues from GitHub and persists them via IssueSyncRepo.
type IssuePoller struct {
	cfg          *config.Github
	githubClient *githubinfra.Client
	logger       *slog.Logger
	syncRepo     IssueSyncRepo
}

func NewIssuePoller(cfg *config.Github, githubClient *githubinfra.Client, logger *slog.Logger, syncRepo IssueSyncRepo) *IssuePoller {
	return &IssuePoller{cfg: cfg, githubClient: githubClient, logger: logger, syncRepo: syncRepo}
}

func (p *IssuePoller) TaskSpec() TaskSpec {
	return TaskSpec{
		Resource: "issue-poller",
		Interval: p.cfg.Issues.ParseInterval,
		Work:     p.Run,
	}
}

func (p *IssuePoller) Run(ctx context.Context, partition Partition) error {
	p.logger.DebugContext(ctx, "polling issues",
		slog.String("owner", partition.Owner),
		slog.String("name", partition.Name),
	)

	cursor, err := p.syncRepo.Cursor(ctx, partition.RepositoryID)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrReadCursor, err)
	}

	resp, err := p.githubClient.GetMostUpdatedIssues(ctx, p.buildRequest(partition, cursor))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFetchIssues, err)
	}

	if resp.NotModified {
		p.logger.InfoContext(ctx, "not modified, skipping")
		p.adaptPollingInterval(ctx)

		return nil
	}

	nextCursor := p.buildNextCursor(resp.NextCursor)

	// Empty page during catch-up: save cursor so page resets to 1.
	// Without this, the poller re-fetches the same empty page forever.
	if len(resp.Issues) == 0 {
		if err := p.syncRepo.Save(ctx, partition.RepositoryID, nil, nextCursor); err != nil {
			return fmt.Errorf("%w: %w", ErrSave, err)
		}

		p.logger.InfoContext(ctx, "no new issues, cursor advanced")
		p.adaptPollingInterval(ctx)

		return nil
	}

	if err := p.syncRepo.Save(ctx, partition.RepositoryID, resp.Issues, nextCursor); err != nil {
		return fmt.Errorf("%w: %w", ErrSave, err)
	}

	p.logger.InfoContext(ctx, "synced", slog.Int("count", len(resp.Issues)))

	return nil
}

func (p *IssuePoller) buildRequest(partition Partition, cursor entities.Cursor) githubinfra.Request {
	req := githubinfra.Request{
		Owner:      partition.Owner,
		Repository: partition.Name,
		Cursor: githubinfra.Cursor{
			Limit: 50,
			Page:  cursor.Page,
			Since: cursor.Since,
		},
	}

	// ETag only in steady-state (page 1). During catch-up ETag is invalid for other pages.
	isCatchUp := cursor.Page > 1
	if !isCatchUp {
		req.Cursor.ETag = cursor.ETag
	}

	return req
}

// buildNextCursor converts a github infrastructure Cursor into a domain Cursor.
func (p *IssuePoller) buildNextCursor(next githubinfra.Cursor) entities.Cursor {
	return entities.Cursor{
		Since: next.Since,
		Page:  next.Page,
		ETag:  next.ETag,
	}
}

func (p *IssuePoller) adaptPollingInterval(_ context.Context) {
	//nolint:godox // noop: will be implemented when adaptive polling is prioritized in https://github.com/thumbrise/autosolve/issues/53
	// TODO: implement exponential backoff on empty responses, reset to base interval when data appears.
}
