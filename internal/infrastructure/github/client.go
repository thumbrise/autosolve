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
	"time"

	"github.com/google/go-github/v84/github"

	"github.com/thumbrise/autosolve/internal/config"
)

var ErrFetchIssues = errors.New("failed fetch issues")

type TransientError error

func NewGithubClient(cfg *config.Github) *github.Client {
	httpClient := &http.Client{
		Transport: http.DefaultTransport,
		Timeout:   cfg.Issues.HttpClientTimeout,
	}

	return github.NewClient(httpClient).WithAuthToken(cfg.Token)
}

type Client struct {
	client *github.Client
	cfg    *config.Github
	logger *slog.Logger
}

func NewClient(cfg *config.Github, client *github.Client, logger *slog.Logger) *Client {
	return &Client{cfg: cfg, client: client, logger: logger}
}

// GetMostUpdatedIssues fetches open issues from the configured repository,
// sorted by update time (oldest first).
//
// Parameters:
//   - count: maximum number of issues to return per page. Values < 1 default to 50.
//   - since: only issues updated after this time are returned. Zero value fetches all.
//
// Errors are wrapped with ErrFetchIssues and classified via mapError:
// transient failures (network, rate limit, 5xx) carry httperr sentinels,
// permanent failures (401, 404, 422) are returned as-is.
// The original GitHub error is preserved in the chain for errors.As access.
//
// On success, resp carries HTTP metadata (rate limit headers, pagination).
// On error, both issues and resp are nil.
func (p *Client) GetMostUpdatedIssues(ctx context.Context, count int, since time.Time) ([]*github.Issue, *github.Response, error) {
	if count < 1 {
		count = 50

		p.logger.WarnContext(ctx, "GetMostUpdatedIssues: count < 1")
	}

	opts := &github.IssueListByRepoOptions{
		State:     "open",
		Sort:      "updated",
		Direction: "asc",
		Since:     since,
		ListOptions: github.ListOptions{
			PerPage: count,
		},
	}

	issues, resp, err := p.client.Issues.ListByRepo(ctx, p.cfg.Owner, p.cfg.Repo, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrFetchIssues, p.mapError(err))
	}

	return issues, resp, nil
}
