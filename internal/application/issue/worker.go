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
	"time"

	"github.com/thumbrise/autosolve/internal/config"
)

type Worker struct {
	cfg    *config.Github
	logger *slog.Logger
	parser *Parser
}

func NewWorker(cfg *config.Github, logger *slog.Logger, parser *Parser) *Worker {
	return &Worker{cfg: cfg, logger: logger, parser: parser}
}

func (w *Worker) Run(ctx context.Context) error {
	logger := w.logger.With(slog.String("component", "issue-parser-worker"))

	logger.InfoContext(ctx, "Starting issue parser worker")

	parser := w.parser

	ticker := time.NewTicker(w.cfg.Issues.ParseInterval)
	defer ticker.Stop()

	poll := func() error {
		logger.InfoContext(ctx, "time to poll new issues")

		n, err := parser.Run(ctx)
		if err != nil {
			return fmt.Errorf("fail parser run: %w", err)
		}

		logger.InfoContext(ctx, fmt.Sprintf("parsed %d issues, waiting %s for next iteration", n, w.cfg.Issues.ParseInterval))

		return nil
	}

	// Poll immediately on startup.
	if err := poll(); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			logger.InfoContext(ctx, "context done")

			return nil
		case <-ticker.C:
			if err := poll(); err != nil {
				return err
			}
		}
	}
}
