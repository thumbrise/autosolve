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

package logger

import (
	"context"
	"log/slog"
	"strings"

	slogmulti "github.com/samber/slog-multi"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// SpanContextHandler is a slog.Handler middleware that automatically
// extracts trace_id, span_id and instrumentation scope from the active
// span in context and adds them to every log record.
//
// Eliminates the need for manual logger.With(...) per component —
// span context is already propagated via ctx.
type SpanContextHandler struct {
	inner slog.Handler
}

func NewSpanContextHandler(inner slog.Handler) *SpanContextHandler {
	return &SpanContextHandler{inner: inner}
}

func (h *SpanContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *SpanContextHandler) Handle(ctx context.Context, record slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		sc := span.SpanContext()
		record.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()[:4]),
			slog.String("span_id", sc.SpanID().String()[:4]),
		)
		// InstrumentationScope is only available via SDK's ReadOnlySpan.
		if roSpan, ok := span.(sdktrace.ReadOnlySpan); ok {
			name := roSpan.InstrumentationScope().Name
			if i := strings.LastIndex(name, "/"); i >= 0 {
				name = name[i+1:]
			}

			record.AddAttrs(
				slog.String("scope", name),
				slog.String("span", roSpan.Name()),
			)
		}
	}

	return h.inner.Handle(ctx, record)
}

func (h *SpanContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SpanContextHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *SpanContextHandler) WithGroup(name string) slog.Handler {
	return &SpanContextHandler{inner: h.inner.WithGroup(name)}
}

// WithOtelBridge extends the given logger with an OTEL slog bridge handler via fanout.
//
// The otelslog handler is created WITHOUT an explicit LoggerProvider — it resolves
// global.GetLoggerProvider() on every log call. This means:
//   - Before telemetry.New → global provider is noop → logs silently discarded by OTEL handler
//   - After telemetry.New calls global.SetLoggerProvider → logs start flowing to collector
//
// This allows the logger to be fully configured in EarlyBootstrap (before any network I/O),
// while the actual OTEL export activates later when the Wire graph creates Telemetry.
func WithOtelBridge(logger *slog.Logger, serviceName string) *slog.Logger {
	existingHandler := logger.Handler()
	otelHandler := otelslog.NewHandler(serviceName, otelslog.WithSource(true))

	return slog.New(slogmulti.Fanout(existingHandler, otelHandler))
}
