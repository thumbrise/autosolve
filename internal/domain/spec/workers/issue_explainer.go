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

package workers

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/thumbrise/autosolve/internal/domain/spec"
	"github.com/thumbrise/autosolve/internal/domain/spec/tenants"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/sqlcgen"
	"github.com/thumbrise/autosolve/internal/infrastructure/ollama"
)

const (
	outboxTopicIssuesSynced = "issues:synced"
	outboxBatchLimit        = 20
)

type IssueExplainer struct {
	db      *sql.DB
	queries *sqlcgen.Queries
	ollama  *ollama.Client
	logger  *slog.Logger
}

func NewIssueExplainer(db *sql.DB, queries *sqlcgen.Queries, ollamaClient *ollama.Client, logger *slog.Logger) *IssueExplainer {
	return &IssueExplainer{db: db, queries: queries, ollama: ollamaClient, logger: logger}
}

func (ie *IssueExplainer) TaskSpec() spec.WorkerSpec {
	return spec.WorkerSpec{
		Resource: "issue-explainer",
		Interval: 5 * time.Second,
		Work:     ie.Run,
	}
}

func (ie *IssueExplainer) Run(ctx context.Context, tenant tenants.RepositoryTenant) error {
	events, err := ie.queries.PendingOutboxEvents(ctx, ie.db, sqlcgen.PendingOutboxEventsParams{
		Topic:        outboxTopicIssuesSynced,
		RepositoryID: tenant.RepositoryID,
		Limit:        outboxBatchLimit,
	})
	if err != nil {
		return fmt.Errorf("read outbox: %w", err)
	}

	if len(events) == 0 {
		return nil
	}

	ie.logger.InfoContext(ctx, "processing outbox events", slog.Int("count", len(events)))

	for _, ev := range events {
		if err := ie.processEvent(ctx, ev); err != nil {
			ie.logger.ErrorContext(ctx, "failed to process event",
				slog.Int64("eventId", ev.ID),
				slog.Int64("issueNumber", ev.ResourceID),
				slog.Any("error", err),
			)

			continue
		}
	}

	return nil
}

func (ie *IssueExplainer) processEvent(ctx context.Context, ev sqlcgen.PendingOutboxEventsRow) error {
	issue, err := ie.queries.GetIssueByRepoAndNumber(ctx, ie.db, sqlcgen.GetIssueByRepoAndNumberParams{
		RepositoryID: ev.RepositoryID,
		Number:       ev.ResourceID,
	})
	if err != nil {
		return fmt.Errorf("get issue #%d: %w", ev.ResourceID, err)
	}

	prompt := fmt.Sprintf(
		"Classify this GitHub issue. Suggest priority (critical/high/medium/low) and component.\n\nTitle: %s\nBody: %s",
		issue.Title,
		issue.Body,
	)

	aiResponse, err := ie.ollama.Generate(ctx, prompt)
	if err != nil {
		return fmt.Errorf("ollama generate: %w", err)
	}

	ie.logger.InfoContext(ctx, "AI analysis complete",
		slog.Int64("issue", issue.Number),
		slog.String("title", issue.Title),
		slog.String("aiResponse", aiResponse),
	)

	if err := ie.queries.AckOutboxEvent(ctx, ie.db, ev.ID); err != nil {
		return fmt.Errorf("ack event %d: %w", ev.ID, err)
	}

	return nil
}
