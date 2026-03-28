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
	"errors"
	"fmt"
	"log/slog"

	"github.com/thumbrise/autosolve/internal/domain/entities"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/sqlcgen"
)

var (
	ErrJobNotFound = errors.New("job not found")
	ErrCreateJob   = errors.New("create job")
	ErrMarkJob     = errors.New("mark job")
)

// JobRepository persists AI dispatch jobs and manages their status lifecycle.
type JobRepository struct {
	db      *sql.DB
	queries *sqlcgen.Queries
	logger  *slog.Logger
}

func NewJobRepository(db *sql.DB, queries *sqlcgen.Queries, logger *slog.Logger) *JobRepository {
	return &JobRepository{db: db, queries: queries, logger: logger}
}

// Create inserts a new job in pending status and returns the created entity.
func (r *JobRepository) Create(ctx context.Context, repositoryID, issueID int64, jobType, prompt string, model *string) (entities.Job, error) {
	ctx, span := tracer.Start(ctx, "JobRepository.Create")
	defer span.End()

	row, err := r.queries.CreateJob(ctx, r.db, sqlcgen.CreateJobParams{
		RepositoryID: repositoryID,
		IssueID:      issueID,
		Type:         jobType,
		Prompt:       prompt,
		Model:        model,
	})
	if err != nil {
		return entities.Job{}, fmt.Errorf("%w: %w", ErrCreateJob, err)
	}

	r.logger.DebugContext(ctx, "job created",
		slog.Int64("jobId", row.ID),
		slog.String("type", jobType),
		slog.Int64("issueId", issueID),
	)

	return entities.Job{
		Record:       entities.Record{ID: row.ID, CreatedAt: row.CreatedAt},
		RepositoryID: repositoryID,
		IssueID:      issueID,
		Type:         jobType,
		Status:       entities.JobStatusPending,
		Prompt:       prompt,
		Model:        model,
	}, nil
}

// MarkProcessing transitions a job from pending to processing.
func (r *JobRepository) MarkProcessing(ctx context.Context, jobID int64) error {
	ctx, span := tracer.Start(ctx, "JobRepository.MarkProcessing")
	defer span.End()

	if err := r.queries.MarkProcessing(ctx, r.db, jobID); err != nil {
		return fmt.Errorf("%w: processing: %w", ErrMarkJob, err)
	}

	return nil
}

// MarkDone transitions a job from processing to done with the AI result.
func (r *JobRepository) MarkDone(ctx context.Context, jobID int64, result string) error {
	ctx, span := tracer.Start(ctx, "JobRepository.MarkDone")
	defer span.End()

	if err := r.queries.MarkDone(ctx, r.db, sqlcgen.MarkDoneParams{
		ID:     jobID,
		Result: &result,
	}); err != nil {
		return fmt.Errorf("%w: done: %w", ErrMarkJob, err)
	}

	return nil
}

// MarkFailed transitions a job from processing to failed with the error message.
func (r *JobRepository) MarkFailed(ctx context.Context, jobID int64, errMsg string) error {
	ctx, span := tracer.Start(ctx, "JobRepository.MarkFailed")
	defer span.End()

	if err := r.queries.MarkFailed(ctx, r.db, sqlcgen.MarkFailedParams{
		ID:        jobID,
		LastError: &errMsg,
	}); err != nil {
		return fmt.Errorf("%w: failed: %w", ErrMarkJob, err)
	}

	return nil
}
