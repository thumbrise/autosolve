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

// Package otel provides an OpenTelemetry [resilience.Plugin] for the resilience package.
//
// This package depends on the OTEL SDK. The core resilience package has zero
// external dependencies — OTEL integration is opt-in via this sub-package.
//
// Import with alias to avoid conflict with the OTEL SDK package:
//
//	import rsotel "github.com/thumbrise/autosolve/pkg/resilience/otel"
package otel

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/thumbrise/autosolve/pkg/resilience"
)

const otelLibrary = "github.com/thumbrise/autosolve/pkg/resilience"

var meter = otel.Meter(otelLibrary)

var (
	callTotal, _    = meter.Int64Counter("resilience.call.total", metric.WithDescription("Total fn calls (including retries)"))
	callDuration, _ = meter.Float64Histogram("resilience.call.duration_seconds", metric.WithDescription("Duration of each fn call"))
	callErrors, _   = meter.Int64Counter("resilience.call.errors", metric.WithDescription("Total fn calls that returned an error"))
	retryTotal, _   = meter.Int64Counter("resilience.retry.total", metric.WithDescription("Total retry decisions (labels: option)"))
	retryWait, _    = meter.Float64Histogram("resilience.retry.wait_seconds", metric.WithDescription("Time spent waiting before retry (labels: option)"))
)

// plugin implements [resilience.Plugin] with OTEL metrics.
type plugin struct{}

// Plugin returns a [resilience.Plugin] that emits OTEL metrics on every
// pipeline lifecycle step.
//
// Metrics:
//   - resilience.call.total — every fn call (including retries)
//   - resilience.call.duration_seconds — duration of each fn call
//   - resilience.call.errors — fn calls that returned an error
//   - resilience.retry.total — retry decisions (labels: option)
//   - resilience.retry.wait_seconds — backoff wait duration (labels: option)
//
// Usage:
//
//	import rsotel "github.com/thumbrise/autosolve/pkg/resilience/otel"
//
//	client := resilience.NewClient(rsotel.Plugin())
func Plugin() resilience.Plugin {
	return &plugin{}
}

// Name returns the plugin identifier.
func (p *plugin) Name() string {
	return "otel"
}

// Events returns OTEL lifecycle hooks.
func (p *plugin) Events() resilience.Events {
	return resilience.Events{
		OnAfterCall: func(ctx context.Context, _ int, err error, duration time.Duration) {
			callTotal.Add(ctx, 1)
			callDuration.Record(ctx, duration.Seconds())

			if err != nil {
				callErrors.Add(ctx, 1)
			}
		},
		OnBeforeWait: func(ctx context.Context, option string, _ int, wait time.Duration) {
			attrs := metric.WithAttributes(attribute.String("option", option))

			retryTotal.Add(ctx, 1, attrs)
			retryWait.Record(ctx, wait.Seconds(), attrs)
		},
	}
}
