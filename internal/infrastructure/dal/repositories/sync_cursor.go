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

	"github.com/thumbrise/autosolve/internal/domain/entities"
	"github.com/thumbrise/autosolve/internal/domain/spec/resources"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/sqlcgen"
)

type SyncCursorRepository struct {
	db      *sql.DB
	queries *sqlcgen.Queries
	logger  *slog.Logger
}

func NewSyncCursorRepository(db *sql.DB, queries *sqlcgen.Queries, logger *slog.Logger) *SyncCursorRepository {
	return &SyncCursorRepository{db: db, queries: queries, logger: logger}
}

// Find returns the cursor for the given repository and resource.
// Returns sql.ErrNoRows when no cursor exists yet — caller should handle via dal.IsNotFound.
func (r *SyncCursorRepository) Find(ctx context.Context, repositoryID int64, resource resources.Resource) (entities.SyncCursor, error) {
	row, err := r.queries.GetSyncCursor(ctx, r.db, sqlcgen.GetSyncCursorParams{
		RepositoryID: repositoryID,
		ResourceType: string(resource),
	})
	if err != nil {
		return entities.SyncCursor{}, err
	}

	return entities.SyncCursor{
		RepositoryID:   row.RepositoryID,
		ResourceType:   row.ResourceType,
		SinceUpdatedAt: row.SinceUpdatedAt,
		NextPage:       int(row.NextPage),
		ETag:           row.ETag,
	}, nil
}

// Save persists the cursor using upsert (insert or update on conflict).
func (r *SyncCursorRepository) Save(ctx context.Context, cursor entities.SyncCursor) error {
	return r.queries.UpsertSyncCursor(ctx, r.db, sqlcgen.UpsertSyncCursorParams{
		RepositoryID:   cursor.RepositoryID,
		ResourceType:   cursor.ResourceType,
		SinceUpdatedAt: cursor.SinceUpdatedAt,
		NextPage:       int64(cursor.NextPage),
		ETag:           cursor.ETag,
	})
}
