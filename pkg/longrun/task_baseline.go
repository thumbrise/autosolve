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

// baselineFailureHandler implements failureHandler for the Baseline classification pipeline.
// It classifies errors via transport + user classifier, selects a Policy, and retries.
type baselineFailureHandler struct {
	baseline *Baseline
	taskName string
	attempts AttemptStore
	logger   *slog.Logger
}

func (h *baselineFailureHandler) Handle(ctx context.Context, err error) error {
	class, policy := h.classify(err)

	if policy == nil {
		// Unknown + no Default → not our problem, skip to let pipeline return permanent.
		return errSkip
	}

	_, knownCategory := h.baseline.Policies[class.Category]

	isDegraded := !knownCategory
	if isDegraded {
		h.logger.ErrorContext(ctx, "DEGRADED: unknown error, retrying with degraded policy",
			slog.Any("error", err),
		)
	}

	return h.retry(ctx, err, policy, class.Category, class.WaitDuration, isDegraded)
}

// classify runs the classification pipeline and returns the
// ErrorClass and the matching Policy. Returns nil policy when the error
// is unknown and Baseline.Default is nil.
func (h *baselineFailureHandler) classify(err error) (*ErrorClass, *Policy) {
	// [1] Built-in transport classify.
	if class := ClassifyTransport(err); class != nil {
		return class, h.policyFor(class.Category)
	}

	// [2] User classifier.
	if h.baseline.Classify != nil {
		if class := h.baseline.Classify(err); class != nil {
			return class, h.policyFor(class.Category)
		}
	}

	// [3] Unknown — no classifier matched.
	unknown := &ErrorClass{Category: CategoryUnknown}

	return unknown, h.baseline.Default // Default may be nil → skip
}

// policyFor returns the policy for the given category.
// Falls back to Baseline.Default when the category has no explicit policy.
func (h *baselineFailureHandler) policyFor(cat ErrorCategory) *Policy {
	if p, ok := h.baseline.Policies[cat]; ok {
		return &p
	}

	return h.baseline.Default
}

// retry retries using a baseline Policy.
func (h *baselineFailureHandler) retry(ctx context.Context, err error, p *Policy, category ErrorCategory, waitOverride time.Duration, isDegraded bool) error {
	key := "baseline:" + categoryName(category)
	attempt := h.attempts.Increment(key)

	if p.Retries > 0 && h.attempts.Get(key) >= p.Retries {
		return err // budget exhausted
	}

	categoryLabel := categoryName(category)

	taskAttr := attribute.String("task", h.taskName)
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

		h.logger.Log(ctx, level, "retrying after explicit wait",
			slog.Any("error", err),
			slog.Any("wait", waitDur),
			slog.Int("attempt", attempt+1),
		)
	} else {
		waitDur = p.Backoff(attempt)

		h.logger.Log(ctx, level, "retrying with backoff",
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
