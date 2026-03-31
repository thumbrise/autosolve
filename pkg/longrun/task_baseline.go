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

package longrun

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// handleBaselineFailure classifies err via baseline and retries accordingly.
func (t *Task) handleBaselineFailure(ctx context.Context, err error) error {
	class, policy := t.classifyWithBaseline(err)

	if policy == nil {
		// Unknown + no Default → permanent error.
		return err
	}

	_, knownCategory := t.baseline.Policies[class.Category]

	isDegraded := !knownCategory
	if isDegraded {
		t.logger.ErrorContext(ctx, "DEGRADED: unknown error, retrying with degraded policy",
			slog.Any("error", err),
		)
	}

	return t.retryWithPolicy(ctx, err, policy, class.Category, class.WaitDuration, isDegraded)
}

// classifyWithBaseline runs the classification pipeline and returns the
// ErrorClass and the matching Policy. Returns nil policy when the error
// is unknown and Baseline.Default is nil.
func (t *Task) classifyWithBaseline(err error) (*ErrorClass, *Policy) {
	// [1] Built-in transport classify.
	if class := ClassifyTransport(err); class != nil {
		return class, t.policyFor(class.Category)
	}

	// [2] User classifier.
	if t.baseline.Classify != nil {
		if class := t.baseline.Classify(err); class != nil {
			return class, t.policyFor(class.Category)
		}
	}

	// [3] Unknown — no classifier matched.
	unknown := &ErrorClass{Category: CategoryUnknown}

	return unknown, t.baseline.Default // Default may be nil → permanent
}

// policyFor returns the policy for the given category.
// Falls back to Baseline.Default when the category has no explicit policy.
func (t *Task) policyFor(cat ErrorCategory) *Policy {
	if p, ok := t.baseline.Policies[cat]; ok {
		return &p
	}

	return t.baseline.Default
}

// retryWithPolicy retries using a baseline Policy.
// category is used to track per-category attempt count for exponential backoff.
// When waitOverride > 0, sleeps exactly that duration instead of backoff.
func (t *Task) retryWithPolicy(ctx context.Context, err error, p *Policy, category ErrorCategory, waitOverride time.Duration, isDegraded bool) error {
	//nolint:godox // retry budget tracking deferred — baseline policies retry indefinitely for now (zero-value = unlimited).
	// TODO: track per-policy retry budget (Policy.Retries). See #121.
	key := "baseline:" + categoryName(category)
	attempt := t.attempts.Increment(key)

	categoryLabel := categoryName(category)

	taskAttr := attribute.String("task", t.name)
	categoryAttr := attribute.String("category", categoryLabel)

	metricBaselineRetryTotal.Add(ctx, 1, metric.WithAttributes(taskAttr, categoryAttr))

	if isDegraded {
		metricDegradedTotal.Add(ctx, 1, metric.WithAttributes(taskAttr))
	}

	level := slog.LevelInfo
	if isDegraded {
		level = slog.LevelError
	}

	var waitDur time.Duration

	if waitOverride > 0 {
		waitDur = waitOverride

		t.logger.Log(ctx, level, "retrying after explicit wait",
			slog.Any("error", err),
			slog.Any("wait", waitDur),
			slog.Int("attempt", attempt+1),
		)
	} else {
		waitDur = p.Backoff(attempt)

		t.logger.Log(ctx, level, "retrying with backoff",
			slog.Any("error", err),
			slog.Any("backoff", waitDur),
			slog.Int("attempt", attempt+1),
		)
	}

	start := time.Now()

	sleepCtx(ctx, waitDur)

	if isDegraded {
		metricDegradedDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(taskAttr))
	}

	return nil
}

// categoryName returns a human-readable label for metrics and logs.
func categoryName(c ErrorCategory) string {
	switch c {
	case CategoryUnknown:
		return "degraded"
	case CategoryNode:
		return "node"
	case CategoryService:
		return "service"
	default:
		return fmt.Sprintf("category_%d", c)
	}
}
