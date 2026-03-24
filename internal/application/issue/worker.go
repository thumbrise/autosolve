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

package issue

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/thumbrise/autosolve/internal/config"
	"github.com/thumbrise/autosolve/pkg/longrun"
)

type Worker struct {
	cfg    *config.Github
	logger *slog.Logger
	parser *Parser
}

func NewWorker(cfg *config.Github, logger *slog.Logger, parser *Parser) *Worker {
	return &Worker{cfg: cfg, logger: logger, parser: parser}
}

// Task returns a longrun.Task that polls issues on the configured interval.
func (w *Worker) Task() *longrun.Task {
	return longrun.NewIntervalTask("polling issues", w.cfg.Issues.ParseInterval, w.poll, []longrun.TransientRule{
		{Err: ErrFetchIssues, MaxRetries: longrun.UnlimitedRetries, Backoff: longrun.DefaultBackoff()},
		{Err: ErrStoreIssues, MaxRetries: longrun.UnlimitedRetries, Backoff: longrun.DefaultBackoff()},
	}, longrun.WithLogger(w.logger))
}

func (w *Worker) poll(ctx context.Context) error {
	w.logger.InfoContext(ctx, "time to poll new issues")

	n, err := w.parser.Run(ctx)
	if err != nil {
		return fmt.Errorf("poll: %w", err)
	}

	w.logger.InfoContext(ctx, fmt.Sprintf("parsed %d issues", n))

	return nil
}
