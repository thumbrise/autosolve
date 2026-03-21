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
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/go-github/v84/github"
)

type Parser struct {
	client *github.Client
	owner  string
	repo   string
}

func NewParser(client *github.Client, owner string, repo string) *Parser {
	return &Parser{client: client, owner: owner, repo: repo}
}

func (f *Parser) Run(ctx context.Context) (int, error) {
	logger := slog.With(slog.String("component", "issue-parser"))

	opts := &github.IssueListByRepoOptions{
		State:     "open",
		Sort:      "updated",
		Direction: "asc",
		Since:     f.lastUpdateTime(),
		ListOptions: github.ListOptions{
			PerPage: 50,
		},
	}

	issues, _, err := f.client.Issues.ListByRepo(ctx, f.owner, f.repo, opts)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch issues: %w", err)
	}

	if len(issues) == 0 {
		logger.DebugContext(ctx, "no new issues found")

		return 0, nil
	}

	slog.DebugContext(ctx, "fetched", slog.Int("count", len(issues)))

	err = f.store(issues)
	if err != nil {
		return 0, err
	}

	slog.DebugContext(ctx, "stored")

	return len(issues), nil
}

func (f *Parser) store(issues []*github.Issue) error {
	indent, err := json.MarshalIndent(issues, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(indent))

	return nil
}

func (f *Parser) lastUpdateTime() time.Time {
	res, err := time.Parse(time.DateOnly, "1999-01-01")
	if err != nil {
		panic(err.Error())
	}

	return res
}
