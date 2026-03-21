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
	"net/http"
	"time"

	"github.com/google/go-github/v84/github"

	"github.com/thumbrise/autosolve/internal/infrastructure/config"
)

type Worker struct{}

func NewWorker() *Worker {
	return &Worker{}
}

func (p *Worker) Run(ctx context.Context) error {
	logger := slog.With(slog.String("component", "issue-parser-worker"))

	logger.DebugContext(ctx, "Starting issue parser worker")

	var cfg Config

	err := config.Read(ctx, &cfg, "github")
	if err != nil {
		return err
	}

	httpClient := &http.Client{
		Transport: http.DefaultTransport,
		Timeout:   cfg.Issues.HttpClientTimeout,
	}

	githubClient := github.NewClient(httpClient).WithAuthToken(cfg.Token)
	parser := NewParser(githubClient, cfg.Owner, cfg.Repo)

	ticker := time.NewTicker(cfg.Issues.ParseInterval)
	defer ticker.Stop()

	poll := func() error {
		logger.DebugContext(ctx, "time to poll new issues")

		n, err := parser.Run(ctx)
		if err != nil {
			return fmt.Errorf("fail parser run: %w", err)
		}

		logger.DebugContext(ctx, fmt.Sprintf("parsed %d issues, waiting %s for next iteration", n, cfg.Issues.ParseInterval))

		return nil
	}

	// Poll immediately on startup.
	if err := poll(); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			logger.DebugContext(ctx, "context done")

			return nil
		case <-ticker.C:
			if err := poll(); err != nil {
				return err
			}
		}
	}
}
