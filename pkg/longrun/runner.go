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
	logger    *slog.Logger
}

func NewRunner(logger *slog.Logger) *Runner {
	return &Runner{logger: logger}
}

type (
	ProcessFunc func(ctx context.Context) error
	Process     struct {
		Name     string
		Start    ProcessFunc
		Shutdown ProcessFunc
	}
)

func shutdownNoOp(ctx context.Context) error {
	return nil
}

// Add registers a process for concurrent execution.
//
// Panics if Process.Start is nil.
// If Process.Name is empty, a warning is logged — named processes improve log readability.
// If Process.Shutdown is nil, a no-op shutdown is assigned.
func (r *Runner) Add(ctx context.Context, process *Process) {
	if process.Name == "" {
		r.logger.WarnContext(ctx, "Runner.Add process name is empty. Consider to add not empty name for clear logs observation")
	}

	if process.Start == nil {
		panic("Runner.Add start function is nil")
	}

	if process.Shutdown == nil {
		r.logger.DebugContext(ctx, "Runner.Add shutdown function is nil, defaulting to no-op")

		process.Shutdown = shutdownNoOp
	}

	r.processes = append(r.processes, process)

	r.logger.DebugContext(ctx, "Runner.Add process added",
		slog.Any("name", process.Name),
	)
}

func (r *Runner) Wait(ctx context.Context) error {
	r.logger.InfoContext(ctx, "runner starting")

	ctxNotified, notifiedCancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer notifiedCancel()

	grp, ctxGrp := errgroup.WithContext(ctxNotified)
	r.startProcesses(ctxGrp, r.processes, grp)
	// Run shutdown in a standalone goroutine so it is not part of the
	// errgroup.  This avoids a deadlock: grp.Wait() blocks until all
	// errgroup goroutines finish, and ctxGrp is cancelled only when
	// Wait returns (or a goroutine errors).  If the shutdown watcher
	// were inside the errgroup and every Start returned nil without a
	// signal, it would block on <-ctxGrp.Done() forever.
	shutdownDone := make(chan struct{})

	go func() {
		defer close(shutdownDone)

		<-ctxGrp.Done()
		r.logger.InfoContext(ctx, "runner context cancelled, shutting down processes")

		ctxShutdown, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer shutdownCancel()

		r.shutdownProcesses(ctxShutdown, r.processes)
	}()

	r.logger.InfoContext(ctx, "runner waiting for processes")

	err := grp.Wait()

	<-shutdownDone

	if err != nil && !errors.Is(err, context.Canceled) {
		r.logger.ErrorContext(ctx, "runner error", slog.Any("error", err))

		return err
	}

	r.logger.InfoContext(ctx, "runner finished")

	return nil
}

func (r *Runner) startProcesses(ctx context.Context, processes []*Process, grp *errgroup.Group) {
	for _, p := range processes {
		logger := r.logger.With(
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
		logger := r.logger.With(
			slog.String("process", p.Name),
		)

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
