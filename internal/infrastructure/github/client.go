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
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/go-github/v84/github"

	"github.com/thumbrise/autosolve/internal/config"
	"github.com/thumbrise/autosolve/internal/domain/entities"
)

var ErrUnexpectedNilResponse = errors.New("go-github returned nil response without error")

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
// The client is stateless per repository — owner and repo come from Request.
// Rate limiting is handled transparently by the underlying http.RoundTripper.
//
// Returns a domain-facing Response that never exposes go-github types.
// Response.NotModified is true when the server returned 304 (ETag matched).
//
// Errors are classified via mapError:
// rate limits → RateLimitError (implements apierr.Retryable, WaitHinted, ServicePressure),
// server errors (5xx) → ServerError (implements apierr.Retryable),
// transport errors → returned as-is (classified by longrun built-in transport classifier),
// permanent failures (401, 404, 422) → returned as-is.
func (p *Client) GetMostUpdatedIssues(ctx context.Context, request Request) (Response, error) {
	ctx, span := tracer.Start(ctx, "Client.GetMostUpdatedIssues")
	defer span.End()

	ctx = withETagContext(ctx, request.Cursor.ETag)

	count := request.Cursor.Limit
	since := request.Cursor.Since
	page := request.Cursor.Page

	if count < 1 {
		count = 50

		p.logger.WarnContext(ctx, "GetMostUpdatedIssues: count < 1")
	}

	if page < 1 {
		page = 1
	}

	opts := &github.IssueListByRepoOptions{
		State:     "all",
		Sort:      "updated",
		Direction: "asc",
		Since:     since,
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: count,
		},
	}

	issues, resp, err := p.githubClient.Issues.ListByRepo(ctx, request.Owner, request.Repository, opts)
	p.writeMetrics(ctx, resp)

	// go-github returns a non-nil error on 304 Not Modified.
	// Check resp first — 304 is not an error, it means data is unchanged and rate limit is not consumed.
	if resp != nil && resp.StatusCode == http.StatusNotModified {
		return Response{NotModified: true}, nil
	}

	if err != nil {
		return Response{}, p.mapError(err)
	}

	if resp == nil {
		return Response{}, ErrUnexpectedNilResponse
	}

	domainIssues, err := p.domainMapper.MapIssues(issues)
	if err != nil {
		return Response{}, fmt.Errorf("map issues: %w", err)
	}

	return Response{
		Issues:     domainIssues,
		NextCursor: p.buildNextCursor(request.Cursor, domainIssues, resp),
	}, nil
}

// buildNextCursor computes the cursor for the next request based on pagination state.
//
// Two modes:
//   - catch-up: resp.NextPage > 0 → more pages remain, keep same Since, advance Page, clear ETag.
//   - steady-state: last page reached → advance Since to last issue's UpdatedAt, reset Page to 1, store ETag.
func (p *Client) buildNextCursor(prev Cursor, issues []*entities.Issue, resp *github.Response) Cursor {
	if resp.NextPage > 0 {
		return Cursor{
			Since: prev.Since,
			Page:  resp.NextPage,
			Limit: prev.Limit,
			ETag:  "",
		}
	}

	newSince := prev.Since
	if len(issues) > 0 {
		newSince = issues[len(issues)-1].GithubUpdatedAt
	}

	return Cursor{
		Since: newSince,
		Page:  1,
		Limit: prev.Limit,
		ETag:  resp.Header.Get("ETag"),
	}
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
	if resp == nil {
		return
	}

	metricRateLimitRemains.Record(ctx, int64(resp.Rate.Remaining))
	metricRateLimitUsed.Record(ctx, int64(resp.Rate.Used))
}
