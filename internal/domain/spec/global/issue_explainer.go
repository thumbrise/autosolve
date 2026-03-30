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

package global

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"maragu.dev/goqite"

	"github.com/thumbrise/autosolve/internal/config"
	"github.com/thumbrise/autosolve/internal/domain/spec"
	"github.com/thumbrise/autosolve/internal/domain/spec/repository"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/sqlcgen"
	githubinfra "github.com/thumbrise/autosolve/internal/infrastructure/github"
	"github.com/thumbrise/autosolve/internal/infrastructure/ollama"
	"github.com/thumbrise/autosolve/internal/infrastructure/queue"
)

const explainerOtelLibrary = "github.com/thumbrise/autosolve/internal/domain/spec/global/issue_explainer"

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

	// metricJobsSkipped counts jobs skipped without processing.
	// Labels: type (job type), reason ("missing_label", "already_responded").
	metricJobsSkipped, _ = explainerMeter.Int64Counter("explainer.jobs.skipped")

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

	// autosolveMarker is an HTML comment embedded in every bot-posted comment.
	// Used to detect feedback loops: if any comment on an issue contains this
	// marker, the explainer skips processing to avoid infinite re-triggering.
	autosolveMarker = "<!-- autosolve -->"
)

// IssueExplainer consumes "issue-explain" jobs from the goqite queue,
// sends issue content to Ollama for AI analysis, and posts the result
// as a comment on the corresponding GitHub issue.
//
// It is a global worker — not multiplied per repository.
// The queue is shared; each JobMessage already contains repositoryId.
type IssueExplainer struct {
	cfg     *config.Github
	db      *sql.DB
	queries *sqlcgen.Queries
	queue   *queue.Queue
	ollama  *ollama.Client
	github  *githubinfra.Client
	logger  *slog.Logger
}

func NewIssueExplainer(cfg *config.Github, db *sql.DB, queries *sqlcgen.Queries, queue *queue.Queue, ollamaClient *ollama.Client, githubClient *githubinfra.Client, logger *slog.Logger) *IssueExplainer {
	return &IssueExplainer{cfg: cfg, db: db, queries: queries, queue: queue, ollama: ollamaClient, github: githubClient, logger: logger}
}

func (e *IssueExplainer) TaskSpec() spec.GlobalTaskSpec {
	return spec.GlobalTaskSpec{
		Resource: "issue-explainer",
		Interval: issueExplainerInterval,
		Work:     e.Run,
	}
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
	case repository.JobTypeIssueExplain:
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

	repo, err := e.queries.GetRepositoryByID(ctx, e.db, job.RepositoryID)
	if err != nil {
		metricJobsFailed.Add(ctx, 1, typeAttr)

		return fmt.Errorf("load repository %d: %w", job.RepositoryID, err)
	}

	skip, reason, err := e.shouldSkip(ctx, repo, issue, job)
	if err != nil {
		metricJobsFailed.Add(ctx, 1, typeAttr)

		return err
	}

	if skip {
		metricJobsSkipped.Add(ctx, 1, typeAttr,
			metric.WithAttributes(attribute.String("reason", reason)))

		return e.queue.Delete(ctx, msgID)
	}

	if err := e.analyzeAndPost(ctx, repo, issue, job, typeAttr); err != nil {
		metricJobsFailed.Add(ctx, 1, typeAttr)

		return err
	}

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

// shouldSkip returns true when the issue must not be analyzed.
// Reasons: missing required label, or already responded (marker found in comments).
// The second return value is the skip reason for metrics (empty when not skipped).
func (e *IssueExplainer) shouldSkip(ctx context.Context, repo sqlcgen.GetRepositoryByIDRow, issue sqlcgen.GetIssueByIDRow, job queue.JobMessage) (bool, string, error) {
	if label := e.cfg.Issues.RequiredLabel; label != "" {
		labels, err := e.github.GetIssueLabels(ctx, repo.Owner, repo.Name, int(issue.Number))
		if err != nil {
			return false, "", fmt.Errorf("get labels for issue %d: %w", job.IssueID, err)
		}

		if !slices.ContainsFunc(labels, func(l string) bool {
			return strings.EqualFold(l, label)
		}) {
			e.logger.InfoContext(ctx, "skipping issue, missing required label",
				slog.Int64("issueId", job.IssueID),
				slog.Int64("issueNumber", issue.Number),
				slog.String("requiredLabel", label),
			)

			return true, "missing_label", nil
		}
	}

	alreadyResponded, err := e.github.HasCommentWithMarker(ctx, repo.Owner, repo.Name, int(issue.Number), autosolveMarker)
	if err != nil {
		return false, "", fmt.Errorf("check marker for issue %d: %w", job.IssueID, err)
	}

	if alreadyResponded {
		e.logger.InfoContext(ctx, "skipping issue, already responded",
			slog.Int64("issueId", job.IssueID),
			slog.Int64("issueNumber", issue.Number),
		)

		return true, "already_responded", nil
	}

	return false, "", nil
}

// analyzeAndPost sends the issue to Ollama and posts the response as a GitHub comment.
func (e *IssueExplainer) analyzeAndPost(ctx context.Context, repo sqlcgen.GetRepositoryByIDRow, issue sqlcgen.GetIssueByIDRow, job queue.JobMessage, typeAttr metric.MeasurementOption) error {
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
		return fmt.Errorf("ollama generate for issue %d: %w", job.IssueID, err)
	}

	e.logger.DebugContext(ctx, "ollama response received",
		slog.Int64("issueId", job.IssueID),
		slog.Int64("issueNumber", issue.Number),
		slog.Int("responseLen", len(response)),
		slog.Duration("elapsed", ollamaElapsed),
		slog.String("response", response),
	)

	commentBody := formatComment(e.ollama.Model(), response)

	// Guard against duplicate comments on message redelivery (#177).
	// If the process crashed after posting but before queue.Delete,
	// the marker will already be present — skip posting.
	alreadyPosted, err := e.github.HasCommentWithMarker(ctx, repo.Owner, repo.Name, int(issue.Number), autosolveMarker)
	if err != nil {
		return fmt.Errorf("idempotency check for issue %d: %w", job.IssueID, err)
	}

	if alreadyPosted {
		e.logger.WarnContext(ctx, "duplicate suppressed, comment already exists",
			slog.Int64("issueId", job.IssueID),
			slog.Int64("issueNumber", issue.Number),
		)

		return nil
	}

	if err := e.github.CreateIssueComment(ctx, repo.Owner, repo.Name, int(issue.Number), commentBody); err != nil {
		return fmt.Errorf("post comment for issue %d: %w", job.IssueID, err)
	}

	e.logger.InfoContext(ctx, "comment posted",
		slog.Int64("issueId", job.IssueID),
		slog.Int64("issueNumber", issue.Number),
		slog.String("repo", repo.Owner+"/"+repo.Name),
	)

	return nil
}

// formatComment builds the Markdown comment body with the autosolve marker.
func formatComment(model, response string) string {
	return fmt.Sprintf("%s\n## AI Analysis\n**Model:** %s\n\n---\n\n%s\n\n---\n\n_Generated by autosolve_",
		autosolveMarker, model, response)
}
