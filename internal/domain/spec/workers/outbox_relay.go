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
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"

	"github.com/thumbrise/autosolve/internal/domain/spec"
	"github.com/thumbrise/autosolve/internal/domain/spec/tenants"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/sqlcgen"
)

const relayOtelLibrary = "github.com/thumbrise/autosolve/internal/domain/spec/workers/outbox_relay"

var relayMeter = otel.Meter(relayOtelLibrary)

var (
	// metricRelayEventsRelayed counts events successfully relayed to the queue.
	metricRelayEventsRelayed, _ = relayMeter.Int64Counter("relay.events.relayed")

	// metricRelayEventsFailed counts events that failed to relay.
	metricRelayEventsFailed, _ = relayMeter.Int64Counter("relay.events.failed")

	// metricRelayBatchSize records how many events were in each batch tick.
	metricRelayBatchSize, _ = relayMeter.Int64Histogram("relay.batch.size")

	// metricRelayBatchDuration records time per batch tick in seconds.
	metricRelayBatchDuration, _ = relayMeter.Float64Histogram("relay.batch.duration_seconds")

	// metricRelayEventAge records lag between outbox event creation and relay in seconds.
	metricRelayEventAge, _ = relayMeter.Float64Histogram("relay.event.age_seconds")

	// metricRelayBatchFull counts how often a batch hits the limit (backpressure signal).
	metricRelayBatchFull, _ = relayMeter.Int64Counter("relay.batch.full")
)

var (
	ErrUnknownTopic = errors.New("unknown outbox topic")
	ErrReadOutbox   = errors.New("read outbox")
)

const (
	outboxTopicIssuesSynced = "issues:synced"
	outboxBatchLimit        = 20

	jobTypeIssueExplain = "issue-explain"
)

// topicJobType maps outbox topics to job types. Extend when new topics appear.
var topicJobType = map[string]string{
	outboxTopicIssuesSynced: jobTypeIssueExplain,
}

// JobQueue is the interface OutboxRelay uses to enqueue work.
// Implemented by infrastructure/queue.Queue.
type JobQueue interface {
	Send(ctx context.Context, jobType string, repositoryID, issueID int64) error
}

// OutboxRelay reads outbox events and enqueues jobs for downstream processors.
// It does not know what happens with the job — only that an event should become a job.
type OutboxRelay struct {
	db      *sql.DB
	queries *sqlcgen.Queries
	queue   JobQueue
	logger  *slog.Logger
}

func NewOutboxRelay(db *sql.DB, queries *sqlcgen.Queries, queue JobQueue, logger *slog.Logger) *OutboxRelay {
	return &OutboxRelay{db: db, queries: queries, queue: queue, logger: logger}
}

func (r *OutboxRelay) TaskSpec() spec.WorkerSpec {
	return spec.WorkerSpec{
		Resource: "outbox-relay",
		Interval: 5 * time.Second,
		Work:     r.Run,
	}
}

func (r *OutboxRelay) Run(ctx context.Context, tenant tenants.RepositoryTenant) error {
	start := time.Now()

	events, err := r.queries.PendingOutboxEvents(ctx, r.db, sqlcgen.PendingOutboxEventsParams{
		Topic:        outboxTopicIssuesSynced,
		RepositoryID: tenant.RepositoryID,
		Limit:        outboxBatchLimit,
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrReadOutbox, err)
	}

	if len(events) == 0 {
		r.logger.DebugContext(ctx, "outbox empty, nothing to relay",
			slog.String("topic", outboxTopicIssuesSynced),
		)

		return nil
	}

	batchFull := len(events) == outboxBatchLimit

	r.logger.InfoContext(ctx, "relaying outbox events to jobs",
		slog.Int("count", len(events)),
		slog.Int("batchLimit", outboxBatchLimit),
		slog.Bool("batchFull", batchFull),
	)

	metricRelayBatchSize.Record(ctx, int64(len(events)))

	if batchFull {
		metricRelayBatchFull.Add(ctx, 1)
	}

	relayed, failed := r.relayBatch(ctx, events)

	metricRelayEventsRelayed.Add(ctx, int64(relayed))
	metricRelayBatchDuration.Record(ctx, time.Since(start).Seconds())

	r.logger.InfoContext(ctx, "relay batch complete",
		slog.Int("relayed", relayed),
		slog.Int("failed", failed),
		slog.Int("total", len(events)),
		slog.Duration("elapsed", time.Since(start)),
	)

	if ctx.Err() != nil {
		return ctx.Err() //nolint:wrapcheck // context cancellation, not a domain error
	}

	return nil
}

// relayBatch encapsulate batch loop.
//
// returns relayed, failed.
func (r *OutboxRelay) relayBatch(ctx context.Context, events []sqlcgen.PendingOutboxEventsRow) (int, int) {
	relayed, failed := 0, 0

	for _, ev := range events {
		if ctx.Err() != nil {
			return relayed, failed
		}

		if err := r.relayEvent(ctx, ev); err != nil {
			failed++

			metricRelayEventsFailed.Add(ctx, 1)

			r.logger.ErrorContext(ctx, "failed to relay event",
				slog.Int64("eventId", ev.ID),
				slog.Int64("resourceId", ev.ResourceID),
				slog.Any("error", err),
			)

			continue
		}

		relayed++
	}

	return relayed, failed
}

func (r *OutboxRelay) relayEvent(ctx context.Context, ev sqlcgen.PendingOutboxEventsRow) error {
	jobType, ok := topicJobType[ev.Topic]

	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownTopic, ev.Topic)
	}

	r.logger.DebugContext(ctx, "relay: resolving issue",
		slog.Int64("eventId", ev.ID),
		slog.String("topic", ev.Topic),
		slog.Int64("resourceId", ev.ResourceID),
	)

	issue, err := r.queries.GetIssueByRepoAndNumber(ctx, r.db, sqlcgen.GetIssueByRepoAndNumberParams{
		RepositoryID: ev.RepositoryID,
		Number:       ev.ResourceID,
	})
	if err != nil {
		return fmt.Errorf("get issue #%d: %w", ev.ResourceID, err)
	}

	r.logger.DebugContext(ctx, "relay: sending to queue",
		slog.Int64("eventId", ev.ID),
		slog.String("type", jobType),
		slog.Int64("issueId", issue.ID),
		slog.String("issueTitle", issue.Title),
	)

	// Send and Ack are not wrapped in a transaction because the DB pool has
	// MaxOpenConns=1 and goqite.Send uses the same *sql.DB internally.
	// BeginTx would hold the only connection, causing goqite.Send to deadlock.
	// The idempotency guard in analyzeAndPost (#177) handles the rare case
	// where Send succeeds but Ack fails on crash — duplicate jobs are harmless.
	if err := r.queue.Send(ctx, jobType, ev.RepositoryID, issue.ID); err != nil {
		return fmt.Errorf("enqueue job for issue #%d: %w", ev.ResourceID, err)
	}

	if err := r.queries.AckOutboxEvent(ctx, r.db, ev.ID); err != nil {
		return fmt.Errorf("ack event %d: %w", ev.ID, err)
	}

	eventAge := time.Since(ev.CreatedAt)
	metricRelayEventAge.Record(ctx, eventAge.Seconds())

	r.logger.DebugContext(ctx, "relay: event processed",
		slog.Int64("eventId", ev.ID),
		slog.String("type", jobType),
		slog.Int64("issueId", issue.ID),
		slog.Duration("eventAge", eventAge),
	)

	return nil
}
