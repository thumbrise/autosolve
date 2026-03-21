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
)

type Loader struct{}

func NewLoader() *Loader {
	return &Loader{}
}

// Load reconfigures the logger level atomically.
// Safe to call at any time — all existing *slog.Logger references
// will immediately reflect the new level via the shared package-level LevelVar.
func (c *Loader) Load(ctx context.Context, debug bool) {
	if debug {
		level.Set(slog.LevelDebug)
	}

	slog.DebugContext(ctx, "logger loaded", slog.Bool("debug", debug))
}
