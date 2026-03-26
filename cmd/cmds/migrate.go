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

package cmds

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"

	"github.com/thumbrise/autosolve/internal/infrastructure/database"
)

var (
	ErrInvalidSteps    = errors.New("steps must be a positive number")
	ErrInvalidDownArgs = errors.New("down requires a positive number or *")
)

// Migrate is a proxy command that dispatches goose operations.
type Migrate struct {
	*cobra.Command
}

func NewMigrate() *Migrate {
	root := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration commands (goose proxy)",
	}

	return &Migrate{root}
}

type (
	MigrateUp      struct{ *cobra.Command }
	MigrateUpFresh struct{ *cobra.Command }
	MigrateDown    struct{ *cobra.Command }
	MigrateStatus  struct{ *cobra.Command }
	MigrateCreate  struct{ *cobra.Command }
	MigrateRedo    struct{ *cobra.Command }
)

func NewMigrateUp(migrator *database.Migrator, logger *slog.Logger) *MigrateUp {
	cmd := &cobra.Command{
		Use:   "up [steps]",
		Short: "Apply pending migrations (all by default)",
		Long:  "Apply pending migrations. Optionally pass step count.\nRequires --yes or interactive confirmation.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirmed(cmd, "Apply pending migrations?") {
				cmd.Println("aborted")

				return nil
			}

			steps, err := parseSteps(args)
			if err != nil {
				return err
			}

			results, err := migrator.Up(cmd.Context(), steps)
			printResults(cmd, logger, results)

			if err != nil {
				return err
			}

			if len(results) == 0 {
				cmd.Println("no pending migrations")
			}

			return nil
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return &MigrateUp{cmd}
}

func NewMigrateUpFresh(migrator *database.Migrator, logger *slog.Logger) *MigrateUpFresh {
	cmd := &cobra.Command{
		Use:   "up:fresh",
		Short: "Drop all tables and re-run all migrations",
		Long:  "Rolls back all migrations, then applies all from scratch.\nRequires --yes or interactive confirmation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirmed(cmd, "Drop ALL tables and re-run ALL migrations?") {
				cmd.Println("aborted")

				return nil
			}

			downResults, upResults, err := migrator.Fresh(cmd.Context())
			printResults(cmd, logger, downResults)
			printResults(cmd, logger, upResults)

			if err != nil {
				return err
			}

			cmd.Println("fresh migration complete")

			return nil
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return &MigrateUpFresh{cmd}
}

func NewMigrateDown(migrator *database.Migrator, logger *slog.Logger) *MigrateDown {
	cmd := &cobra.Command{
		Use:   "down <steps|*>",
		Short: "Roll back migrations",
		Long:  "Roll back migrations. Requires step count or * for all.\nRequires --yes or interactive confirmation.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == "*" {
				if !confirmed(cmd, "Roll back ALL migrations?") {
					cmd.Println("aborted")

					return nil
				}

				results, err := migrator.DownAll(cmd.Context())
				printResults(cmd, logger, results)

				return err
			}

			steps, err := strconv.Atoi(args[0])
			if err != nil || steps <= 0 {
				return fmt.Errorf("%w: got %q", ErrInvalidDownArgs, args[0])
			}

			if !confirmed(cmd, fmt.Sprintf("Roll back %d migration(s)?", steps)) {
				cmd.Println("aborted")

				return nil
			}

			results, err := migrator.Down(cmd.Context(), steps)
			printResults(cmd, logger, results)

			return err
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return &MigrateDown{cmd}
}

func NewMigrateStatus(migrator *database.Migrator) *MigrateStatus {
	return &MigrateStatus{&cobra.Command{
		Use:   "status",
		Short: "Show migration status",
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := migrator.Status(cmd.Context())
			if err != nil {
				return err
			}

			for _, r := range results {
				cmd.Printf("%-10s %s\n", r.State, r.Source.Path)
			}

			return nil
		},
	}}
}

func NewMigrateCreate(migrator *database.Migrator) *MigrateCreate {
	return &MigrateCreate{&cobra.Command{
		Use:   "create <name>",
		Short: "Create a new SQL migration file",
		Long:  "Creates a new SQL migration file on disk.\nExample: go run . migrate create add_users",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := migrator.Create(args[0])
			if err != nil {
				return err
			}

			cmd.Printf("created %s\n", path)

			return nil
		},
	}}
}

func NewMigrateRedo(migrator *database.Migrator, logger *slog.Logger) *MigrateRedo {
	cmd := &cobra.Command{
		Use:   "redo",
		Short: "Roll back the last migration, then re-apply it",
		Long:  "Equivalent to down 1 + up 1.\nRequires --yes or interactive confirmation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirmed(cmd, "Redo the last migration?") {
				cmd.Println("aborted")

				return nil
			}

			downResults, upResults, err := migrator.Redo(cmd.Context())
			printResults(cmd, logger, downResults)
			printResults(cmd, logger, upResults)

			return err
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return &MigrateRedo{cmd}
}

// confirmed checks --yes flag or prompts interactively.
func confirmed(cmd *cobra.Command, prompt string) bool {
	yes, _ := cmd.Flags().GetBool("yes")
	if yes {
		return true
	}

	cmd.PrintErr(prompt + " [y/N] ")

	scanner := bufio.NewScanner(cmd.InOrStdin())
	if !scanner.Scan() {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(scanner.Text()), "y")
}

func parseSteps(args []string) (int, error) {
	if len(args) == 0 {
		return 0, nil
	}

	steps, err := strconv.Atoi(args[0])
	if err != nil || steps <= 0 {
		return 0, fmt.Errorf("%w: got %q", ErrInvalidSteps, args[0])
	}

	return steps, nil
}

// printResults prints migration results to CLI output and emits structured log
// entries so migration operations are observable via the telemetry pipeline.
// Safe to call with nil or empty slice.
func printResults(cmd *cobra.Command, logger *slog.Logger, results []*goose.MigrationResult) {
	for _, r := range results {
		cmd.Println(r.String())
		logger.InfoContext(cmd.Context(), "migration executed",
			slog.String("direction", r.Direction),
			slog.String("file", r.Source.Path),
			slog.String("duration", r.Duration.String()),
		)
	}
}
