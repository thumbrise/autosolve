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
	"fmt"
	"log/slog"
	"time"

	"github.com/google/go-github/v84/github"

	"github.com/thumbrise/autosolve/internal/config"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/model"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/repositories"
)

type Parser struct {
	githubClient    *github.Client
	logger          *slog.Logger
	issueRepository *repositories.IssueRepository
	cfg             *config.Github
}

func NewParser(cfg *config.Github, githubClient *github.Client, issueRepository *repositories.IssueRepository, logger *slog.Logger) *Parser {
	return &Parser{cfg: cfg, githubClient: githubClient, issueRepository: issueRepository, logger: logger}
}

func (p *Parser) Run(ctx context.Context) (int, error) {
	logger := p.logger.With(slog.String("component", "issue-parser"))

	opts, err := p.options(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to build options: %w", err)
	}

	logger.DebugContext(ctx, "starting request to list issues",
		"opts", opts,
	)

	issues, _, err := p.githubClient.Issues.ListByRepo(ctx, p.cfg.Owner, p.cfg.Repo, opts)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch issues: %w", err)
	}

	if len(issues) == 0 {
		logger.InfoContext(ctx, "no new issues found")

		return 0, nil
	}

	logger.InfoContext(ctx, "fetched", slog.Int("count", len(issues)))

	err = p.store(ctx, issues)
	if err != nil {
		return 0, err
	}

	logger.InfoContext(ctx, "issues stored", slog.Int("count", len(issues)))

	return len(issues), nil
}

func (p *Parser) options(ctx context.Context) (*github.IssueListByRepoOptions, error) {
	opts := &github.IssueListByRepoOptions{
		State:             "open",
		Sort:              "updated",
		Direction:         "asc",
		ListCursorOptions: github.ListCursorOptions{},
		ListOptions: github.ListOptions{
			PerPage: 50,
		},
	}

	since, err := p.optionSince(ctx)
	if err != nil {
		return nil, err
	}

	opts.Since = since

	return opts, nil
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
		IssueID:         issue.GetID(),
		Title:           issue.GetTitle(),
		Body:            issue.GetBody(),
		State:           state,
		Status:          model.IssueProcessingStatusPending,
		GithubCreatedAt: issue.GetCreatedAt().Time,
		GithubUpdatedAt: issue.GetUpdatedAt().Time,
		SyncedAt:        &now,
	}

	labels := make([]*model.IssueLabel, 0, len(issue.Labels))
	for _, gl := range issue.Labels {
		l := &model.IssueLabel{
			ID:          gl.GetID(),
			URL:         gl.GetURL(),
			Name:        gl.GetName(),
			Color:       gl.GetColor(),
			Description: gl.GetDescription(),
			NodeID:      gl.GetNodeID(),
			Default:     gl.GetDefault(),
		}
		labels = append(labels, l)
	}

	result.Labels = labels

	assignees := make([]*model.IssueAssignee, 0, len(issue.Assignees))
	for _, ga := range issue.Assignees {
		a := &model.IssueAssignee{
			ID:    ga.GetID(),
			Login: ga.GetLogin(),
		}
		assignees = append(assignees, a)
	}

	result.Assignees = assignees

	return result
}

func (p *Parser) optionSince(ctx context.Context) (time.Time, error) {
	res, err := p.issueRepository.GetLastUpdateTime(ctx)
	if err != nil {
		if dal.IsNotFound(err) {
			return time.Time{}, nil
		}

		return time.Time{}, err
	}

	return res.Add(1 * time.Second), nil
}
