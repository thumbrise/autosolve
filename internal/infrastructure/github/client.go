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

package github

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/go-github/v84/github"

	"github.com/thumbrise/autosolve/internal/config"
	"github.com/thumbrise/autosolve/internal/domain/entities"
)

type Client struct {
	githubClient *github.Client
	logger       *slog.Logger
	domainMapper *DomainMapper
}

func NewClient(logger *slog.Logger, cfg *config.Github, transport *Transport, domainMapper *DomainMapper) *Client {
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   cfg.HttpClientTimeout,
	}

	githubClient := github.NewClient(httpClient).WithAuthToken(cfg.Token)

	return &Client{
		githubClient: githubClient,
		logger:       logger,
		domainMapper: domainMapper,
	}
}

// GetMostUpdatedIssues fetches issues from the given repository,
// sorted by update time (oldest first).
//
// The githubClient is stateless per repository — owner and repo are explicit parameters.
// Rate limiting is handled transparently by the underlying http.RoundTripper.
//
// Parameters:
//   - owner: repository owner (e.g. "thumbrise").
//   - repo: repository name (e.g. "autosolve").
//   - count: maximum number of issues to return per page. Values < 1 default to 50.
//   - since: only issues updated after this time are returned. Zero value fetches all.
//
// Errors are classified via mapError:
// transient failures (network, rate limit, 5xx) carry httperr sentinels,
// permanent failures (401, 404, 422) are returned as-is.
// The original GitHub error is preserved in the chain for errors.As access.
//
// On success, resp carries HTTP metadata (rate limit headers, pagination).
// On error, both issues and resp are nil.
func (p *Client) GetMostUpdatedIssues(ctx context.Context, request Request) ([]*entities.Issue, *github.Response, error) {
	ctx, span := tracer.Start(ctx, "Client.GetMostUpdatedIssues")
	defer span.End()

	ctx = withETagContext(ctx, request.Cursor.ETag)

	count := request.Cursor.Limit
	since := request.Cursor.Since

	if count < 1 {
		count = 50

		p.logger.WarnContext(ctx, "GetMostUpdatedIssues: count < 1")
	}

	opts := &github.IssueListByRepoOptions{
		State:     "all",
		Sort:      "updated",
		Direction: "asc",
		Since:     since,
		ListOptions: github.ListOptions{
			PerPage: count,
		},
	}

	issues, resp, err := p.githubClient.Issues.ListByRepo(ctx, request.Owner, request.Repository, opts)
	if err != nil {
		return nil, nil, p.mapError(err)
	}

	p.writeMetrics(ctx, resp)

	domainIssues, err := p.domainMapper.MapIssues(issues)
	if err != nil {
		return nil, nil, fmt.Errorf("map issues: %w", err)
	}
	// TODO: replace response. isolate domain from google/go-github
	return domainIssues, resp, nil
}

// GetRepository fetches repository metadata from GitHub API.
// Used by preflight to validate repository existence and accessibility.
func (p *Client) GetRepository(ctx context.Context, owner, repo string) (*github.Repository, error) {
	r, _, err := p.githubClient.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, p.mapError(err)
	}

	return r, nil
}

func (p *Client) writeMetrics(ctx context.Context, resp *github.Response) {
	metricRateLimitRemains.Record(ctx, int64(resp.Rate.Remaining))
	metricRateLimitUsed.Record(ctx, int64(resp.Rate.Used))
}
