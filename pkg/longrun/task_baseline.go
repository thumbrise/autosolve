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

	"github.com/thumbrise/autosolve/pkg/resilience"
)

// baselineOption creates a resilience.Option that implements the Baseline
// classification pipeline. It classifies errors via transport + user classifier,
// selects a Policy, and retries with the appropriate backoff.
//
// This Option sits after task-level retry Options in the middleware chain.
// If a task-level retry.On already handled the error (retried and succeeded,
// or retried and exhausted budget), baseline never sees it.
// Baseline only sees errors that no task-level rule matched.
func baselineOption(baseline *Baseline, taskName string, logger *slog.Logger) resilience.Option {
	return resilience.NewOption(func(next resilience.Func) resilience.Func {
		var attempt int

		return func(ctx context.Context) error {
			for {
				err := next(ctx)
				if err == nil {
					attempt = 0

					return nil
				}

				if ctx.Err() != nil {
					return ctx.Err()
				}

				class, policy := classifyError(baseline, err)
				if policy == nil {
					// Unknown + no Default → permanent.
					return err
				}

				// Budget check: Policy.Retries > 0 → exact limit.
				// Policy.Retries <= 0 → unlimited (baseline default).
				if policy.Retries > 0 && attempt >= policy.Retries {
					logger.ErrorContext(ctx, "baseline: max retries reached",
						slog.Any("error", err),
						slog.Int("max_retries", policy.Retries),
					)

					return err
				}

				_, knownCategory := baseline.Policies[class.Category]
				isDegraded := !knownCategory

				if isDegraded {
					logger.ErrorContext(ctx, "DEGRADED: unknown error, retrying with degraded policy",
						slog.Any("error", err),
					)
				}

				catLabel := categoryName(class.Category)

				level := slog.LevelInfo
				if isDegraded {
					level = slog.LevelError
				}

				var waitDur time.Duration
				if class.WaitDuration > 0 {
					waitDur = class.WaitDuration
				} else {
					waitDur = policy.Backoff(attempt)
				}

				start := time.Now()

				logger.Log(ctx, level, "retrying with baseline policy",
					slog.Any("error", err),
					slog.String("category", catLabel),
					slog.Any("backoff", waitDur),
				)

				attempt++

				sleepCtx(ctx, waitDur)

				if ctx.Err() != nil {
					return ctx.Err()
				}

				taskAttr := attribute.String("task", taskName)
				categoryAttr := attribute.String("category", catLabel)

				metricBaselineRetryTotal.Add(ctx, 1, metric.WithAttributes(taskAttr, categoryAttr))

				if isDegraded {
					metricDegradedTotal.Add(ctx, 1, metric.WithAttributes(taskAttr))
					metricDegradedDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(taskAttr))
				}
			}
		}
	})
}

// classifyError runs the classification pipeline and returns the
// ErrorClass and the matching Policy. Returns nil policy when the error
// is unknown and Baseline.Default is nil.
func classifyError(baseline *Baseline, err error) (*ErrorClass, *Policy) {
	// [1] Built-in transport classify.
	if class := ClassifyTransport(err); class != nil {
		return class, policyFor(baseline, class.Category)
	}

	// [2] User classifier.
	if baseline.Classify != nil {
		if class := baseline.Classify(err); class != nil {
			return class, policyFor(baseline, class.Category)
		}
	}

	// [3] Unknown — no classifier matched.
	unknown := &ErrorClass{Category: CategoryUnknown}

	return unknown, baseline.Default // Default may be nil → permanent
}

// policyFor returns the policy for the given category.
// Falls back to Baseline.Default when the category has no explicit policy.
func policyFor(baseline *Baseline, cat ErrorCategory) *Policy {
	if p, ok := baseline.Policies[cat]; ok {
		return &p
	}

	return baseline.Default
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
