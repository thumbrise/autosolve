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

	"github.com/m-mizutani/masq"
)

func NewSlogLogger() *slog.Logger {
	// Reading debug flag directly from env to avoid circular dependency:
	// logger must be created before config, but config needs logger.
	debug := os.Getenv("AUTOSOLVE_DEBUG") == "true"

	opts := &slog.HandlerOptions{
		AddSource:   false,
		Level:       slog.LevelInfo,
		ReplaceAttr: masq.New(masq.WithTag("secret", maskPercentHead('*', 75))),
	}

	if debug {
		opts.Level = slog.LevelDebug
		opts.AddSource = true
	}

	l := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(l)

	return l
}
