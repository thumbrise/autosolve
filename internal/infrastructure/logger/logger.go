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
	"os"

	"github.com/m-mizutani/masq"
	slogmulti "github.com/samber/slog-multi"
	"go.opentelemetry.io/contrib/bridges/otelslog"

	"github.com/thumbrise/autosolve/internal/config"
	stringsutil "github.com/thumbrise/autosolve/pkg/strings"
)

// maskReplacer masks the first `percent` percent of the string's runes
// with the given symbol, leaving the rest visible.
func maskReplacer(symbol rune, percent int) func(groups []string, a slog.Attr) slog.Attr {
	redactor := masq.RedactString(func(s string) string {
		return stringsutil.MaskPercent(s, symbol, percent)
	})

	return masq.New(masq.WithTag("secret", redactor))
}

func defaultOptions() *slog.HandlerOptions {
	return &slog.HandlerOptions{
		ReplaceAttr: maskReplacer('*', 75),
	}
}

// New creates basic *slog.Logger with no external dependencies.
// Secret masking (masq) is always active.
// Source info is not collected.
// Level is Info.
// Handler is TextHandler
func New() *slog.Logger {
	opts := &slog.HandlerOptions{
		ReplaceAttr: maskReplacer('*', 75),
	}

	l := slog.New(slog.NewTextHandler(os.Stdout, opts))

	return l
}

func WithConfig(ctx context.Context, cfg config.Log) *slog.Logger {
	opts := defaultOptions()

	if cfg.Debug {
		opts.Level = slog.LevelDebug
	}

	opts.AddSource = cfg.Source

	l := slog.New(slog.NewTextHandler(os.Stdout, opts))

	l.DebugContext(ctx, "logger loaded",
		slog.Any("cfg", cfg),
	)

	return l
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
