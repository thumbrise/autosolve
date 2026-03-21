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

package longrun

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

type Runner struct {
	processes []*Process
}

func NewRunner() *Runner {
	return &Runner{}
}

type (
	ProcessFunc func(ctx context.Context) error
	Process     struct {
		Name     string
		Start    ProcessFunc
		Shutdown ProcessFunc
	}
)

func (r *Runner) Add(process *Process) {
	if process.Name == "" {
		slog.Warn("Runner.Add process name is empty")
	}

	if process.Start == nil {
		panic("Runner.Add start function is nil")
	}

	if process.Shutdown == nil {
		slog.Warn("Runner.Add shutdown function is nil")
	}

	r.processes = append(r.processes, process)
}

func (r *Runner) Wait(ctx context.Context) error {
	slog.InfoContext(ctx, "runner starting")

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	grp, ctx := errgroup.WithContext(ctx)
	r.startProcesses(ctx, r.processes, grp)

	slog.InfoContext(ctx, "runner waiting for processes")

	err := grp.Wait()

	slog.InfoContext(ctx, "runner processes finished, shutting down")

	ctx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	r.shutdownProcesses(ctx, r.processes)

	if err != nil && !errors.Is(err, context.Canceled) {
		slog.ErrorContext(ctx, "runner error", slog.Any("error", err))

		return err
	}

	slog.InfoContext(ctx, "runner finished")

	return nil
}

func (r *Runner) startProcesses(ctx context.Context, processes []*Process, grp *errgroup.Group) {
	for _, p := range processes {
		logger := slog.With(
			slog.String("process", p.Name),
		)

		grp.Go(func() error {
			logger.InfoContext(ctx, "starting process")

			return p.Start(ctx)
		})
	}
}

func (r *Runner) shutdownProcesses(ctx context.Context, processes []*Process) {
	for _, p := range processes {
		logger := slog.With(
			slog.String("process", p.Name),
		)

		if p.Shutdown == nil {
			continue
		}

		err := p.Shutdown(ctx)
		if err != nil {
			logger.ErrorContext(ctx, "failed to shutdown process",
				slog.Any("error", err),
			)

			continue
		}

		logger.InfoContext(ctx, "successfully shutdown")
	}
}
