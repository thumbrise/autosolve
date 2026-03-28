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

package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"maragu.dev/goqite"
)

const (
	// QueueName is the single queue used for all AI dispatch jobs.
	QueueName = "ai-jobs"

	// Timeout is how long a message stays invisible after Receive.
	// If the consumer dies, the message reappears after this duration.
	Timeout = 60 * time.Second

	// MaxReceive is how many times a message can be received before
	// it becomes a dead message and is no longer delivered.
	MaxReceive = 5
)

// ErrSendJob is returned when a message cannot be enqueued.
var ErrSendJob = errors.New("send job")

// JobMessage is the payload serialized into the goqite message body.
type JobMessage struct {
	Type         string `json:"type"`
	RepositoryID int64  `json:"repositoryId"`
	IssueID      int64  `json:"issueId"`
}

// Queue wraps goqite.Queue and implements workers.JobQueue.
type Queue struct {
	q *goqite.Queue
}

// NewQueue creates a goqite-backed job queue.
func NewQueue(db *sql.DB) *Queue {
	return &Queue{
		q: goqite.New(goqite.NewOpts{
			DB:         db,
			Name:       QueueName,
			Timeout:    Timeout,
			MaxReceive: MaxReceive,
		}),
	}
}

// Send enqueues a job message for downstream processing.
// Implements workers.JobQueue.
func (q *Queue) Send(ctx context.Context, jobType string, repositoryID, issueID int64) error {
	body, err := json.Marshal(JobMessage{
		Type:         jobType,
		RepositoryID: repositoryID,
		IssueID:      issueID,
	})
	if err != nil {
		return fmt.Errorf("%w: marshal: %w", ErrSendJob, err)
	}

	if err := q.q.Send(ctx, goqite.Message{Body: body}); err != nil {
		return fmt.Errorf("%w: %w", ErrSendJob, err)
	}

	return nil
}

// Receive dequeues the next message from the queue.
// Returns nil message and nil error when the queue is empty.
func (q *Queue) Receive(ctx context.Context) (*goqite.Message, error) {
	msg, err := q.q.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("receive: %w", err)
	}

	return msg, nil
}

// Delete acknowledges a message, removing it from the queue.
func (q *Queue) Delete(ctx context.Context, id goqite.ID) error {
	if err := q.q.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	return nil
}

// Inner returns the underlying goqite.Queue for use by the job processor (#156).
func (q *Queue) Inner() *goqite.Queue {
	return q.q
}
