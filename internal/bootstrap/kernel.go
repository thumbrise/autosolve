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

package bootstrap

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/thumbrise/autosolve/cmd"
	"github.com/thumbrise/autosolve/internal/infrastructure/telemetry"
)

const envPrefix = "AUTOSOLVE"

type ShutdownFunc func(ctx context.Context) error

type Kernel struct {
	rootCommand *cmd.Root
	commands    []*cobra.Command
	db          *sql.DB
	telemetry   *telemetry.Telemetry
	logger      *slog.Logger
}

func NewKernel(commands []*cobra.Command, logger *slog.Logger, rootCommand *cmd.Root, db *sql.DB, telemetry *telemetry.Telemetry) *Kernel {
	return &Kernel{commands: commands, logger: logger, rootCommand: rootCommand, db: db, telemetry: telemetry}
}

func (b *Kernel) Execute(ctx context.Context, output io.Writer) error {
	b.registerCommands()
	b.rootCommand.SetOut(output)

	defer b.shutdown(ctx, 5*time.Second, b.telemetry.Shutdown)
	defer b.shutdownDB()

	return b.rootCommand.ExecuteContext(ctx)
}

func (b *Kernel) registerCommands() {
	for _, command := range b.commands {
		b.rootCommand.AddCommand(command)
	}
}

func (b *Kernel) shutdownDB() {
	if err := b.db.Close(); err != nil {
		b.logger.Error("failed to close database", slog.Any("error", err))
	}
}

func (b *Kernel) shutdown(ctx context.Context, timeout time.Duration, fn ShutdownFunc) {
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	err := fn(shutdownCtx)
	if err != nil {
		b.logger.ErrorContext(shutdownCtx, "shutdown error",
			slog.Any("error", err),
		)
	}
}
