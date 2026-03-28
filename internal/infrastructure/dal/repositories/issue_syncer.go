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
	"fmt"
	"log/slog"
	"time"

	"github.com/thumbrise/autosolve/internal/domain/entities"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/sqlcgen"
)

// outboxTopicIssuesSynced is the topic written to outbox_events when issues are synced.
// Internal to this repository — consumers of the outbox table will filter by this value.
const outboxTopicIssuesSynced = "issues:synced"

// IssueSyncer is an aggregate root that atomically persists issues, advances the
// poll cursor, and writes outbox events — all in a single SQLite transaction.
type IssueSyncer struct {
	db      *sql.DB
	queries *sqlcgen.Queries
	logger  *slog.Logger
}

func NewIssueSyncer(db *sql.DB, queries *sqlcgen.Queries, logger *slog.Logger) *IssueSyncer {
	return &IssueSyncer{db: db, queries: queries, logger: logger}
}

// Cursor returns the current poll cursor for the given repository.
// Returns zero-value Cursor when no cursor exists yet (first run).
func (s *IssueSyncer) Cursor(ctx context.Context, repositoryID int64) (entities.Cursor, error) {
	ctx, span := tracer.Start(ctx, "IssueSyncer.Cursor")
	defer span.End()

	row, err := s.queries.GetSyncCursor(ctx, s.db, sqlcgen.GetSyncCursorParams{
		RepositoryID: repositoryID,
		Topic:        outboxTopicIssuesSynced,
	})
	if err != nil {
		if dal.IsNotFound(err) {
			s.logger.DebugContext(ctx, "cursor not found, starting from scratch",
				slog.Int64("repositoryId", repositoryID),
			)

			return entities.Cursor{}, nil
		}

		return entities.Cursor{}, fmt.Errorf("get cursor: %w", err)
	}

	s.logger.DebugContext(ctx, "cursor found",
		slog.Int64("repositoryId", repositoryID),
		slog.Time("since", row.SinceUpdatedAt),
		slog.Int64("nextPage", row.NextPage),
		slog.String("etag", row.ETag),
	)

	return entities.Cursor{
		Since: row.SinceUpdatedAt,
		Page:  int(row.NextPage),
		ETag:  row.ETag,
	}, nil
}

// Save atomically upserts issues, advances the poll cursor, and writes outbox events
// in a single transaction. issues may be nil (cursor-only save for empty pages).
func (s *IssueSyncer) Save(ctx context.Context, repositoryID int64, issues []*entities.Issue, cursor entities.Cursor) error {
	ctx, span := tracer.Start(ctx, "IssueSyncer.Save")
	defer span.End()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	if err := s.upsertIssues(ctx, tx, repositoryID, issues); err != nil {
		return err
	}

	if err := s.queries.UpsertSyncCursor(ctx, tx, sqlcgen.UpsertSyncCursorParams{
		RepositoryID:   repositoryID,
		Topic:          outboxTopicIssuesSynced,
		SinceUpdatedAt: cursor.Since,
		NextPage:       int64(cursor.Page),
		ETag:           cursor.ETag,
	}); err != nil {
		return fmt.Errorf("save cursor: %w", err)
	}

	if err := s.writeOutboxEvents(ctx, tx, repositoryID, issues); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	s.logger.DebugContext(ctx, "sync saved",
		slog.Int64("repositoryId", repositoryID),
		slog.Int("issues", len(issues)),
	)

	return nil
}

func (s *IssueSyncer) upsertIssues(ctx context.Context, tx *sql.Tx, repositoryID int64, issues []*entities.Issue) error {
	for _, iss := range issues {
		if err := s.queries.UpsertIssue(ctx, tx, sqlcgen.UpsertIssueParams{
			RepositoryID:    repositoryID,
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
			SyncedAt:        time.Now().UTC(),
		}); err != nil {
			return fmt.Errorf("upsert issue %d: %w", iss.Number, err)
		}
	}

	return nil
}

func (s *IssueSyncer) writeOutboxEvents(ctx context.Context, tx *sql.Tx, repositoryID int64, issues []*entities.Issue) error {
	for _, iss := range issues {
		if err := s.queries.InsertOutboxEvent(ctx, tx, sqlcgen.InsertOutboxEventParams{
			Topic:        outboxTopicIssuesSynced,
			ResourceID:   iss.Number,
			RepositoryID: repositoryID,
		}); err != nil {
			return fmt.Errorf("outbox event for issue %d: %w", iss.Number, err)
		}
	}

	return nil
}
