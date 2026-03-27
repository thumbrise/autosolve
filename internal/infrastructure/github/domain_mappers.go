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
	"time"

	"github.com/google/go-github/v84/github"

	"github.com/thumbrise/autosolve/internal/domain/entities"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/repositories"
)

type DomainMapper struct {
	repoRepo *repositories.RepositoryRepository
}

func NewDomainMapper(repoRepo *repositories.RepositoryRepository) *DomainMapper {
	return &DomainMapper{repoRepo: repoRepo}
}

func (d *DomainMapper) MapIssues(ctx context.Context, req Request, issues []*github.Issue) ([]*entities.Issue, error) {
	repositoryID, err := d.repoRepo.GetIDByOwnerAndName(ctx, req.Owner, req.Repository)
	if err != nil {
		return nil, err
	}

	domainIssues := make([]*entities.Issue, 0, len(issues))
	for _, issue := range issues {
		domainIssues = append(domainIssues, d.MapIssue(repositoryID, issue))
	}

	return domainIssues, nil
}

func (d *DomainMapper) MapIssue(repositoryID int64, issue *github.Issue) *entities.Issue {
	state := entities.IssueStateOpen
	if issue.GetState() == "closed" {
		state = entities.IssueStateClosed
	}

	now := time.Now()

	result := &entities.Issue{
		RepositoryID:    repositoryID,
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
