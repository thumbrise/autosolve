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

package repositories

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/thumbrise/autosolve/internal/infrastructure/dal/model"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/sqlcgen"
)

type IssueRepository struct {
	db      *sql.DB
	queries *sqlcgen.Queries
	logger  *slog.Logger
}

func NewIssueRepository(db *sql.DB, queries *sqlcgen.Queries, logger *slog.Logger) *IssueRepository {
	return &IssueRepository{db: db, queries: queries, logger: logger}
}

func (r *IssueRepository) UpsertMany(ctx context.Context, issues []*model.Issue) error {
	if len(issues) == 0 {
		return nil
	}

	r.logger.DebugContext(ctx, "received issues for persistence", slog.Int("count", len(issues)))

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	for _, iss := range issues {
		err := r.queries.UpsertIssue(ctx, tx, sqlcgen.UpsertIssueParams{
			RepositoryID:    iss.RepositoryID,
			GithubID:        iss.GithubID,
			Number:          iss.Number,
			Title:           iss.Title,
			Body:            iss.Body,
			State:           iss.State,
			IsPullRequest:   iss.IsPullRequest,
			PrUrl:           iss.PRUrl,
			PrHtmlUrl:       iss.PRHtmlUrl,
			PrDiffUrl:       iss.PRDiffUrl,
			PrPatchUrl:      iss.PRPatchUrl,
			GithubCreatedAt: iss.GithubCreatedAt,
			GithubUpdatedAt: iss.GithubUpdatedAt,
			SyncedAt:        iss.SyncedAt,
		})
		if err != nil {
			r.logger.ErrorContext(ctx, "failed to upsert issue", slog.Any("error", err))

			return err
		}
	}

	return tx.Commit()
}

func (r *IssueRepository) GetLastUpdateTime(ctx context.Context, repositoryID int64) (time.Time, error) {
	row, err := r.queries.GetLastUpdateTime(ctx, r.db, repositoryID)
	if err != nil {
		return time.Time{}, err
	}

	r.logger.DebugContext(ctx, "found last update time",
		"last_update_time", row.GithubUpdatedAt,
		"github_id", row.GithubID,
		"repository_id", repositoryID,
	)

	return row.GithubUpdatedAt, nil
}
