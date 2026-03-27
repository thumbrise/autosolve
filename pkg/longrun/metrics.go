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

import "go.opentelemetry.io/otel"

const otelLibrary = "github.com/thumbrise/autosolve/pkg/longrun"

var meter = otel.Meter(otelLibrary)

var (
	// metricDegradedTotal counts each retry in degraded mode.
	// Labels: task (task name).
	metricDegradedTotal, _ = meter.Int64Counter("longrun_degraded_total")

	// metricDegradedDuration records time spent in a single degraded wait (seconds).
	// Labels: task (task name).
	metricDegradedDuration, _ = meter.Float64Histogram("longrun_degraded_duration_seconds")

	// metricBaselineRetryTotal counts each retry via baseline policy (any category).
	// Labels: task (task name), category (node, service, degraded).
	metricBaselineRetryTotal, _ = meter.Int64Counter("longrun_baseline_retry_total")
)
