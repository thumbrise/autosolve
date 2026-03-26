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

	"github.com/spf13/cobra"

	"github.com/thumbrise/autosolve/cmd"
)

const envPrefix = "AUTOSOLVE"

type Kernel struct {
	rootCommand *cmd.Root
	commands    []*cobra.Command
	db          *sql.DB
	logger      *slog.Logger
}

func NewKernel(commands []*cobra.Command, db *sql.DB, logger *slog.Logger, rootCommand *cmd.Root) *Kernel {
	return &Kernel{commands: commands, db: db, logger: logger, rootCommand: rootCommand}
}

func (b *Kernel) Execute(ctx context.Context, output io.Writer) error {
	b.registerCommands()

	b.rootCommand.SetOut(output)
	defer b.shutdownDB(ctx)

	return b.rootCommand.ExecuteContext(ctx)
}

func (b *Kernel) registerCommands() {
	for _, command := range b.commands {
		b.rootCommand.AddCommand(command)
	}
}

func (b *Kernel) shutdownDB(ctx context.Context) {
	if err := b.db.Close(); err != nil {
		b.logger.ErrorContext(ctx, "failed to close database", slog.Any("error", err))
	}
}
