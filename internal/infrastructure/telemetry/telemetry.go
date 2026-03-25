// Copyright 2026 thumbrise
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/thumbrise/autosolve/internal/config"
)

var (
	ErrNewResource       = errors.New("failed to create OTEL resource")
	ErrNewTracerProvider = errors.New("failed to create OTEL tracer provider")
	ErrNewMeterProvider  = errors.New("failed to create OTEL meter provider")
	ErrNewLoggerProvider = errors.New("failed to create OTEL logger provider")
)

// Telemetry owns the OpenTelemetry SDK lifecycle.
//
// Create via New, which builds resource, creates gRPC exporters for all enabled signals,
// registers providers globally via otel.Set* and sets up propagators.
//
// After New returns, use the standard global API anywhere in the application:
//
//	tracer := otel.Tracer("autosolve/mypackage")
//	meter  := otel.Meter("autosolve/mypackage")
//
// Logs are handled via the slog OTEL bridge configured in EarlyBootstrap.
// The bridge resolves global.GetLoggerProvider() lazily, so logs start flowing
// to the collector as soon as this constructor calls global.SetLoggerProvider.
//
// Call Shutdown to flush and release all resources.
type Telemetry struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	loggerProvider *sdklog.LoggerProvider
	logger         *slog.Logger
}

const none = "none"

// New initializes the OpenTelemetry SDK and registers all providers globally.
//
// When cfg.SDKDisabled is true, returns a no-op Telemetry (global providers remain default noop).
func New(ctx context.Context, cfg *config.Otel, logger *slog.Logger) (*Telemetry, error) { //nolint:cyclop
	t := &Telemetry{logger: logger}
	if cfg.SDKDisabled {
		logger.InfoContext(ctx, "OTEL SDK disabled, using noop providers")

		return t, nil
	}

	res, err := newResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNewResource, err)
	}

	if cfg.Traces.Exporter != none && cfg.Traces.Exporter != "" {
		t.tracerProvider, err = newTracerProvider(ctx, cfg, res)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrNewTracerProvider, err)
		}

		otel.SetTracerProvider(t.tracerProvider)
	}

	if cfg.Metrics.Exporter != none && cfg.Metrics.Exporter != "" {
		t.meterProvider, err = newMeterProvider(ctx, cfg, res)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrNewMeterProvider, err)
		}

		otel.SetMeterProvider(t.meterProvider)
	}

	if cfg.Logs.Exporter != none && cfg.Logs.Exporter != "" {
		t.loggerProvider, err = newLoggerProvider(ctx, cfg, res)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrNewLoggerProvider, err)
		}
		// This is the moment the slog OTEL bridge (configured in EarlyBootstrap)
		// starts actually exporting logs. Before this call, the bridge's
		// global.GetLoggerProvider() returned noop.
		global.SetLoggerProvider(t.loggerProvider)
	}

	otel.SetTextMapPropagator(newPropagator(cfg))
	otel.SetErrorHandler(otelErrHandler{logger: logger})
	logger.InfoContext(ctx, "OTEL SDK initialized",
		slog.String("service", cfg.ServiceName),
		slog.String("traces", cfg.Traces.Exporter),
		slog.String("metrics", cfg.Metrics.Exporter),
		slog.String("logs", cfg.Logs.Exporter),
		slog.String("endpoint", cfg.Exporter.Endpoint),
	)

	return t, nil
}

// Shutdown flushes and releases all OTEL SDK resources.
// Shutdown order is reverse of creation: logs → metrics → traces.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs error

	if t.loggerProvider != nil {
		if err := t.loggerProvider.Shutdown(ctx); err != nil {
			errs = errors.Join(errs, fmt.Errorf("otel logger provider shutdown: %w", err))
		}
	}

	if t.meterProvider != nil {
		if err := t.meterProvider.Shutdown(ctx); err != nil {
			errs = errors.Join(errs, fmt.Errorf("otel meter provider shutdown: %w", err))
		}
	}

	if t.tracerProvider != nil {
		if err := t.tracerProvider.Shutdown(ctx); err != nil {
			errs = errors.Join(errs, fmt.Errorf("otel tracer provider shutdown: %w", err))
		}
	}

	if errs != nil {
		t.logger.ErrorContext(ctx, "OTEL SDK shutdown completed with errors", slog.String("errors", errs.Error()))
	} else {
		t.logger.InfoContext(ctx, "OTEL SDK shutdown completed")
	}

	return errs
}

// otelErrHandler routes OTEL internal errors to the application logger.
type otelErrHandler struct {
	logger *slog.Logger
}

func (h otelErrHandler) Handle(err error) {
	h.logger.Error("opentelemetry internal error", slog.String("error", err.Error()))
}
