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

	"github.com/thumbrise/autosolve/internal/infrastructure/dal/sqlcgen"
)

type RepositoryRepository struct {
	db      *sql.DB
	queries *sqlcgen.Queries
	logger  *slog.Logger
}

func NewRepositoryRepository(db *sql.DB, queries *sqlcgen.Queries, logger *slog.Logger) *RepositoryRepository {
	return &RepositoryRepository{db: db, queries: queries, logger: logger}
}

// Upsert inserts or updates a repository by owner+name and returns its local ID.
func (r *RepositoryRepository) Upsert(ctx context.Context, owner, name string) (int64, error) {
	id, err := r.queries.UpsertRepository(ctx, r.db, sqlcgen.UpsertRepositoryParams{
		Owner: owner,
		Name:  name,
	})
	if err != nil {
		return 0, err
	}

	r.logger.DebugContext(ctx, "upserted repository",
		slog.Int64("id", id),
		slog.String("owner", owner),
		slog.String("name", name),
	)

	return id, nil
}

// GetIDByOwnerAndName returns the local repository ID for the given owner and name.
func (r *RepositoryRepository) GetIDByOwnerAndName(ctx context.Context, owner, name string) (int64, error) {
	id, err := r.queries.GetByOwnerName(ctx, r.db, sqlcgen.GetByOwnerNameParams{
		Owner: owner,
		Name:  name,
	})
	if err != nil {
		return 0, err
	}

	return id, nil
}
