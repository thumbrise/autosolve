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
	"log/slog"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/m-mizutani/masq"
)

// maskPercentHead masks the first `percent` percent of the string's runes
// with the given symbol, leaving the rest visible.
func maskPercentHead(symbol rune, percent int) masq.Redactor {
	return masq.RedactString(func(s string) string {
		if percent <= 0 {
			return s
		}

		if percent >= 100 {
			return strings.Repeat(string(symbol), utf8.RuneCountInString(s))
		}

		runes := []rune(s)
		n := len(runes)

		maskCount := n * percent / 100
		if maskCount > n {
			maskCount = n
		}

		return strings.Repeat(string(symbol), maskCount) + string(runes[maskCount:])
	})
}

// level is shared between NewSlogLogger and Loader within this package.
// It allows Loader to change the log level atomically after config is loaded.
var level = &slog.LevelVar{} // INFO by default

// NewSlogLogger creates a fully configured *slog.Logger with no external dependencies.
// Level is controlled via package-level LevelVar and can be changed atomically
// at any time via Loader.Load (e.g. after config is loaded).
// Secret masking (masq) is always active. Source info is collected but only emitted at debug level.
func NewSlogLogger() *slog.Logger {
	masqReplacer := masq.New(masq.WithTag("secret", maskPercentHead('*', 75)))

	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Strip source info when not in debug mode.
			if a.Key == slog.SourceKey && level.Level() > slog.LevelDebug {
				return slog.Attr{}
			}

			return masqReplacer(groups, a)
		},
	}

	l := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(l)

	return l
}
