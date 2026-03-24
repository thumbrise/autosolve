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
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/thumbrise/autosolve/internal/infrastructure/dal/model"
)

type IssueRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

func NewIssueRepository(db *gorm.DB, logger *slog.Logger) *IssueRepository {
	return &IssueRepository{db: db, logger: logger}
}

func (r *IssueRepository) UpsertMany(ctx context.Context, issues []*model.Issue) error {
	if len(issues) == 0 {
		return nil
	}

	r.logger.DebugContext(ctx, "received issues for persistence", slog.Int("count", len(issues)))

	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "issue_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"title", "body", "state",
				"github_created_at", "github_updated_at", "labels", "assignees",
				"updated_at", "synced_at",
			}),
		}).
		CreateInBatches(issues, len(issues)).Error
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to upsert issues", slog.Any("error", err))

		return err
	}

	r.logger.DebugContext(ctx, "persisted issues", slog.Int("count", len(issues)))

	return nil
}

func (r *IssueRepository) GetLastUpdateTime(ctx context.Context) (time.Time, error) {
	m := model.Issue{}

	err := r.db.WithContext(ctx).Select("issue_id", "github_updated_at").Order("github_updated_at desc").First(&m).Error
	if err != nil {
		return time.Time{}, err
	}

	r.logger.DebugContext(ctx, "found last update time",
		"last_update_time", m.GithubUpdatedAt,
		"issue_id", m.IssueID,
	)

	return m.GithubUpdatedAt, nil
}
