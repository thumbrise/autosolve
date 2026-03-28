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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"maragu.dev/goqite"

	"github.com/thumbrise/autosolve/internal/infrastructure/dal/sqlcgen"
	"github.com/thumbrise/autosolve/internal/infrastructure/ollama"
	"github.com/thumbrise/autosolve/internal/infrastructure/queue"
)

const explainerOtelLibrary = "github.com/thumbrise/autosolve/internal/domain/spec/workers/issue_explainer"

var explainerMeter = otel.Meter(explainerOtelLibrary)

var (
	// metricJobsProcessed counts successfully completed jobs.
	// Labels: type (job type, e.g. "issue-explain").
	metricJobsProcessed, _ = explainerMeter.Int64Counter("explainer.jobs.processed")

	// metricJobsFailed counts jobs that failed processing (will be retried by goqite).
	// Labels: type (job type).
	metricJobsFailed, _ = explainerMeter.Int64Counter("explainer.jobs.failed")

	// metricJobsPoison counts poison messages deleted without processing.
	// Labels: reason ("unmarshal", "unknown_type").
	metricJobsPoison, _ = explainerMeter.Int64Counter("explainer.jobs.poison")

	// metricOllamaDuration records Ollama call latency in seconds.
	metricOllamaDuration, _ = explainerMeter.Float64Histogram("explainer.ollama.duration_seconds")

	// metricQueueEmpty counts how often the poll finds an empty queue (idle signal).
	metricQueueEmpty, _ = explainerMeter.Int64Counter("explainer.queue.empty")
)

// ErrExplainIssue is returned when the explainer fails to process a job.
var ErrExplainIssue = errors.New("explain issue")

const (
	// issueExplainerInterval is how often the explainer polls the queue.
	issueExplainerInterval = 2 * time.Second

	// issueExplainPrompt is the default prompt sent to Ollama for issue classification.
	issueExplainPrompt = "Classify this GitHub issue. Suggest priority (critical/high/medium/low) and component."
)

// IssueExplainer consumes "issue-explain" jobs from the goqite queue,
// sends issue content to Ollama for AI analysis, and logs the result.
//
// It is a global worker — not multiplied per repository.
// The queue is shared; each JobMessage already contains repositoryId.
type IssueExplainer struct {
	db      *sql.DB
	queries *sqlcgen.Queries
	queue   *queue.Queue
	ollama  *ollama.Client
	logger  *slog.Logger
}

func NewIssueExplainer(db *sql.DB, queries *sqlcgen.Queries, queue *queue.Queue, ollamaClient *ollama.Client, logger *slog.Logger) *IssueExplainer {
	return &IssueExplainer{db: db, queries: queries, queue: queue, ollama: ollamaClient, logger: logger}
}

// Interval returns the poll interval for use by the scheduler.
func (e *IssueExplainer) Interval() time.Duration {
	return issueExplainerInterval
}

// Run polls the queue once, processes one message if available.
// Returns nil when the queue is empty (normal idle).
// Returns error when processing fails — message stays in queue (visibility timeout).
func (e *IssueExplainer) Run(ctx context.Context) error {
	msg, err := e.queue.Receive(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrExplainIssue, err)
	}

	if msg == nil {
		metricQueueEmpty.Add(ctx, 1)

		return nil // queue empty, nothing to do
	}

	if err := e.processMessage(ctx, msg); err != nil {
		return fmt.Errorf("%w: %w", ErrExplainIssue, err)
	}

	return nil
}

func (e *IssueExplainer) processMessage(ctx context.Context, msg *goqite.Message) error {
	var job queue.JobMessage
	if err := json.Unmarshal(msg.Body, &job); err != nil {
		e.logger.ErrorContext(ctx, "invalid job message, deleting poison message",
			slog.String("body", string(msg.Body)),
			slog.Any("error", err),
		)

		metricJobsPoison.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", "unmarshal")))

		return e.queue.Delete(ctx, msg.ID)
	}

	e.logger.InfoContext(ctx, "processing job",
		slog.String("type", job.Type),
		slog.Int64("repositoryId", job.RepositoryID),
		slog.Int64("issueId", job.IssueID),
	)

	switch job.Type {
	case jobTypeIssueExplain:
		return e.explainIssue(ctx, msg.ID, job)
	default:
		e.logger.WarnContext(ctx, "unknown job type, deleting",
			slog.String("type", job.Type),
		)

		metricJobsPoison.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", "unknown_type")))

		return e.queue.Delete(ctx, msg.ID)
	}
}

func (e *IssueExplainer) explainIssue(ctx context.Context, msgID goqite.ID, job queue.JobMessage) error {
	typeAttr := metric.WithAttributes(attribute.String("type", job.Type))

	issue, err := e.queries.GetIssueByID(ctx, e.db, job.IssueID)
	if err != nil {
		metricJobsFailed.Add(ctx, 1, typeAttr)

		return fmt.Errorf("load issue %d: %w", job.IssueID, err)
	}

	prompt := fmt.Sprintf("%s\n\nTitle: %s\nBody: %s", issueExplainPrompt, issue.Title, issue.Body)

	e.logger.InfoContext(ctx, "calling ollama",
		slog.Int64("issueId", job.IssueID),
		slog.Int64("issueNumber", issue.Number),
		slog.String("model", e.ollama.Model()),
	)

	start := time.Now()

	response, err := e.ollama.Generate(ctx, prompt)

	ollamaElapsed := time.Since(start)
	metricOllamaDuration.Record(ctx, ollamaElapsed.Seconds(), typeAttr)

	if err != nil {
		metricJobsFailed.Add(ctx, 1, typeAttr)

		return fmt.Errorf("ollama generate for issue %d: %w", job.IssueID, err)
	}

	e.logger.InfoContext(ctx, "ollama response received",
		slog.Int64("issueId", job.IssueID),
		slog.Int64("issueNumber", issue.Number),
		slog.Int("responseLen", len(response)),
		slog.Duration("elapsed", ollamaElapsed),
		slog.String("response", response),
	)

	if err := e.queue.Delete(ctx, msgID); err != nil {
		return fmt.Errorf("delete message for issue %d: %w", job.IssueID, err)
	}

	metricJobsProcessed.Add(ctx, 1, typeAttr)

	e.logger.InfoContext(ctx, "job complete",
		slog.Int64("issueId", job.IssueID),
		slog.Int64("issueNumber", issue.Number),
		slog.String("type", job.Type),
	)

	return nil
}
