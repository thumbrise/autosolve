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

package config

import (
	"context"
	"strings"
	"time"

	"github.com/thumbrise/autosolve/internal/infrastructure/config"
)

// Otel holds OpenTelemetry configuration.
//
// Field names follow OTEL SDK environment variable specification.
// See https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/
//
// Viper key: "otel"
//
// Environment variables (with AUTOSOLVE_ prefix):
//
//	AUTOSOLVE_OTEL_SERVICENAME           → otel.serviceName
//	AUTOSOLVE_OTEL_RESOURCEATTRIBUTES    → otel.resourceAttributes
//	AUTOSOLVE_OTEL_SDKDISABLED           → otel.sdkDisabled
//	AUTOSOLVE_OTEL_PROPAGATORS           → otel.propagators
//	AUTOSOLVE_OTEL_TRACES_EXPORTER       → otel.traces.exporter
//	AUTOSOLVE_OTEL_TRACES_SAMPLER        → otel.traces.sampler
//	AUTOSOLVE_OTEL_TRACES_SAMPLERARG     → otel.traces.samplerArg
//	AUTOSOLVE_OTEL_METRICS_EXPORTER      → otel.metrics.exporter
//	AUTOSOLVE_OTEL_LOGS_EXPORTER         → otel.logs.exporter
//	AUTOSOLVE_OTEL_EXPORTER_ENDPOINT     → otel.exporter.endpoint
//	AUTOSOLVE_OTEL_EXPORTER_PROTOCOL     → otel.exporter.protocol
//	AUTOSOLVE_OTEL_EXPORTER_HEADERS      → otel.exporter.headers
//	AUTOSOLVE_OTEL_EXPORTER_TIMEOUT      → otel.exporter.timeout
type Otel struct {
	// ServiceName maps to OTEL_SERVICE_NAME.
	ServiceName string `validate:"required"`
	// ResourceAttributes maps to OTEL_RESOURCE_ATTRIBUTES.
	// Format: "key1=value1,key2=value2"
	ResourceAttributes string
	// SDKDisabled maps to OTEL_SDK_DISABLED. Disables the SDK entirely when true.
	SDKDisabled bool
	// Propagators maps to OTEL_PROPAGATORS.
	// Comma-separated list, e.g. "tracecontext,baggage"
	Propagators string
	Traces      OtelTraces
	Metrics     OtelMetrics
	Logs        OtelLogs
	Exporter    OtelExporter
}

// OtelTraces holds trace-specific settings.
type OtelTraces struct {
	// Exporter maps to OTEL_TRACES_EXPORTER. Values: "otlp", "zipkin", "none".
	Exporter string
	// Sampler maps to OTEL_TRACES_SAMPLER.
	// Values: "always_on", "always_off", "traceidratio", "parentbased_always_on", etc.
	Sampler string
	// SamplerArg maps to OTEL_TRACES_SAMPLER_ARG. E.g. "0.5" for 50% sampling ratio.
	SamplerArg string
}

// OtelMetrics holds metrics-specific settings.
type OtelMetrics struct {
	// Exporter maps to OTEL_METRICS_EXPORTER. Values: "otlp", "prometheus", "none".
	Exporter string
}

// OtelLogs holds logs-specific settings.
type OtelLogs struct {
	// Exporter maps to OTEL_LOGS_EXPORTER. Values: "otlp", "none".
	Exporter string
}

// OtelExporter holds OTLP exporter settings.
type OtelExporter struct {
	// Endpoint maps to OTEL_EXPORTER_OTLP_ENDPOINT.
	Endpoint string
	// Protocol maps to OTEL_EXPORTER_OTLP_PROTOCOL. Values: "grpc", "http/protobuf".
	Protocol string
	// Headers maps to OTEL_EXPORTER_OTLP_HEADERS.
	// Format: "key1=value1,key2=value2"
	Headers string `masq:"secret"`
	// Timeout maps to OTEL_EXPORTER_OTLP_TIMEOUT.
	Timeout time.Duration
}

// ParseHeaders parses Headers string into a map.
//
// Format follows OTEL spec: "key1=value1,key2=value2".
// Uses SplitN to handle values containing "=" (e.g. uptrace DSN with query params).
//
// Returns empty map if Headers is empty.
func (e *OtelExporter) ParseHeaders() map[string]string {
	result := make(map[string]string)
	if e.Headers == "" {
		return result
	}

	for _, pair := range strings.Split(e.Headers, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	return result
}

func NewOtel(ctx context.Context, reader *config.Reader) (*Otel, error) {
	return config.Read[Otel](ctx, reader)
}
