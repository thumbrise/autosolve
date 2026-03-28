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
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/m-mizutani/masq"
	slogmulti "github.com/samber/slog-multi"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/thumbrise/autosolve/internal/config"
	stringsutil "github.com/thumbrise/autosolve/pkg/strings"
)

const logPath = "runtime/logs/autosolve.log"

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

	handlers := make([]slog.Handler, 0, 2)

	fileHandler := slog.NewTextHandler(newFileWriter(logPath), opts)
	handlers = append(handlers, fileHandler)

	if cfg.Terminal {
		terminalHandler := slog.NewTextHandler(os.Stderr, opts)
		handlers = append(handlers, terminalHandler)
	}

	for i, handler := range handlers {
		handlers[i] = NewSpanContextHandler(handler)
	}

	h := slogmulti.Fanout(handlers...)

	l := slog.New(h)

	l.DebugContext(ctx, "logger loaded",
		slog.Any("cfg", cfg),
	)

	return l
}

func newFileWriter(output string) io.Writer {
	if output == "stdout" {
		return os.Stdout
	}

	path := output
	if path == "" {
		path = logPath
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		// Can't create dir — fall back to stdout and hope someone notices.
		_, _ = fmt.Fprintf(os.Stderr, "autosolve: cannot create log dir %s: %v, falling back to stdout\n", dir, err)

		return os.Stdout
	}

	return &lumberjack.Logger{
		Filename:   path,
		MaxSize:    10, // MB
		MaxBackups: 3,
		MaxAge:     28, // days
		Compress:   true,
	}
}
