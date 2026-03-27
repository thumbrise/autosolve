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

package telemetry

import (
	"context"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/thumbrise/autosolve/internal/config"
)

// newResource builds an OTEL resource from config.
//
// Parses ResourceAttributes string ("key1=value1,key2=value2") into attributes.
// Always includes service.name from config. Merges with telemetry SDK, host, and container detectors.
func newResource(ctx context.Context, cfg *config.Otel) (*resource.Resource, error) {
	attrs := parseResourceAttributes(cfg.ResourceAttributes)
	attrs = append(attrs, semconv.ServiceName(cfg.ServiceName))

	return resource.New(ctx,
		resource.WithAttributes(attrs...),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithContainer(),
	)
}

// parseResourceAttributes parses "key1=value1,key2=value2" into []attribute.KeyValue.
func parseResourceAttributes(raw string) []attribute.KeyValue {
	if raw == "" {
		return nil
	}

	var attrs []attribute.KeyValue

	for _, pair := range strings.Split(raw, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) == 2 {
			attrs = append(attrs, attribute.String(
				strings.TrimSpace(parts[0]),
				strings.TrimSpace(parts[1]),
			))
		}
	}

	return attrs
}

// newTracerProvider creates a TracerProvider with a gRPC OTLP exporter.
func newTracerProvider(ctx context.Context, cfg *config.Otel, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Exporter.Endpoint),
		otlptracegrpc.WithHeaders(cfg.Exporter.ParseHeaders()),
		otlptracegrpc.WithTimeout(cfg.Exporter.Timeout),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(newSampler(cfg)),
	), nil
}

// newMeterProvider creates a MeterProvider with a gRPC OTLP exporter.
func newMeterProvider(ctx context.Context, cfg *config.Otel, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	exp, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(cfg.Exporter.Endpoint),
		otlpmetricgrpc.WithHeaders(cfg.Exporter.ParseHeaders()),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)),
	), nil
}

// newLoggerProvider creates a LoggerProvider with a gRPC OTLP exporter.
func newLoggerProvider(ctx context.Context, cfg *config.Otel, res *resource.Resource) (*sdklog.LoggerProvider, error) {
	exp, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(cfg.Exporter.Endpoint),
		otlploggrpc.WithHeaders(cfg.Exporter.ParseHeaders()),
		otlploggrpc.WithTimeout(cfg.Exporter.Timeout),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	return sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exp)),
	), nil
}

// newSampler creates a trace sampler from config.
//
// Supported values for cfg.Traces.Sampler:
//   - "always_on"                → AlwaysSample
//   - "always_off"               → NeverSample
//   - "traceidratio"             → TraceIDRatioBased (uses SamplerArg as ratio, default 1.0)
//   - "parentbased_always_on"    → ParentBased(AlwaysSample)
//   - "parentbased_always_off"   → ParentBased(NeverSample)
//   - "parentbased_traceidratio" → ParentBased(TraceIDRatioBased)
//   - "" or unknown              → ParentBased(AlwaysSample) (OTEL SDK default)
func newSampler(cfg *config.Otel) sdktrace.Sampler {
	ratio := parseSamplerArg(cfg.Traces.SamplerArg)
	switch strings.ToLower(cfg.Traces.Sampler) {
	case "always_on":
		return sdktrace.AlwaysSample()
	case "always_off":
		return sdktrace.NeverSample()
	case "traceidratio":
		return sdktrace.TraceIDRatioBased(ratio)
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	case "parentbased_traceidratio":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	default:
		// "parentbased_always_on" or empty — OTEL SDK default behavior.
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	}
}

// parseSamplerArg parses the sampler argument string to a float64 ratio.
// Returns 1.0 (sample everything) if parsing fails or value is empty.
func parseSamplerArg(arg string) float64 {
	if arg == "" {
		return 1.0
	}

	ratio, err := strconv.ParseFloat(arg, 64)
	if err != nil {
		return 1.0
	}

	return ratio
}

// newPropagator creates a composite TextMapPropagator from config.
//
// Parses cfg.Propagators as comma-separated list. Supported values:
//   - "tracecontext" → W3C TraceContext
//   - "baggage"      → W3C Baggage
//
// Default (empty string): tracecontext + baggage.
func newPropagator(cfg *config.Otel) propagation.TextMapPropagator {
	if cfg.Propagators == "" {
		return propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
	}

	var propagators []propagation.TextMapPropagator

	for _, name := range strings.Split(cfg.Propagators, ",") {
		switch strings.TrimSpace(strings.ToLower(name)) {
		case "tracecontext":
			propagators = append(propagators, propagation.TraceContext{})
		case "baggage":
			propagators = append(propagators, propagation.Baggage{})
		}
	}

	if len(propagators) == 0 {
		return propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
	}

	return propagation.NewCompositeTextMapPropagator(propagators...)
}
