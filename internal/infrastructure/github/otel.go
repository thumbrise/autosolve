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

package github

import "go.opentelemetry.io/otel"

const otelLibrary = "github.com/thumbrise/autosolve/internal/infrastructure/github"

var (
	tracer = otel.Tracer(otelLibrary)
	meter  = otel.Meter(otelLibrary)
)

var (
	metricRateLimitRemains, _  = meter.Int64Gauge("github_api_limit_remains")
	metricRateLimitUsed, _     = meter.Int64Gauge("github_api_limit_used")
	metricRateLimitConsumed, _ = meter.Int64Counter("github_api_limit_consumed")
)
