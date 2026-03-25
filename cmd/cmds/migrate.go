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
	"github.com/spf13/cobra"

	"github.com/thumbrise/autosolve/internal/infrastructure/database"
)

// Migrate is a proxy command that dispatches goose operations.
//
// It hides inconvenient parameters (directory, dialect, DSN) so the developer
// can simply run:
//
//	go run . migrate status
//	go run . migrate up
//	go run . migrate down
//	go run . migrate create add_users sql
type Migrate struct {
	*cobra.Command
}

func NewMigrate(migrator *database.Migrator) *Migrate {
	root := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration commands (goose proxy)",
	}

	root.AddCommand(
		newMigrateUp(migrator),
		newMigrateDown(migrator),
		newMigrateStatus(migrator),
		newMigrateCreate(migrator),
		newMigrateRedo(migrator),
		newMigrateFresh(migrator),
	)

	return &Migrate{root}
}

func newMigrateUp(migrator *database.Migrator) *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := migrator.Up(cmd.Context())
			if err != nil {
				return err
			}

			for _, r := range results {
				cmd.Printf("OK  %s (%s)\n", r.Source.Path, r.Duration)
			}

			if len(results) == 0 {
				cmd.Println("no pending migrations")
			}

			return nil
		},
	}
}

func newMigrateDown(migrator *database.Migrator) *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Roll back the last migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := migrator.Down(cmd.Context())
			if err != nil {
				return err
			}

			if result == nil {
				cmd.Println("no migrations to roll back")

				return nil
			}

			cmd.Printf("ROLLED BACK  %s (%s)\n", result.Source.Path, result.Duration)

			return nil
		},
	}
}

func newMigrateStatus(migrator *database.Migrator) *cobra.Command {
	return &cobra.Command{
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
	}
}

func newMigrateCreate(migrator *database.Migrator) *cobra.Command {
	return &cobra.Command{
		Use:   "create [name]",
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
	}
}

func newMigrateFresh(migrator *database.Migrator) *cobra.Command {
	return &cobra.Command{
		Use:   "fresh",
		Short: "Drop all tables and re-run all migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := migrator.Fresh(cmd.Context())
			if err != nil {
				return err
			}

			for _, r := range results {
				cmd.Printf("OK  %s (%s)\n", r.Source.Path, r.Duration)
			}

			cmd.Println("fresh migration complete")

			return nil
		},
	}
}

func newMigrateRedo(migrator *database.Migrator) *cobra.Command {
	return &cobra.Command{
		Use:   "redo",
		Short: "Roll back the last migration, then re-apply it",
		RunE: func(cmd *cobra.Command, args []string) error {
			downResult, upResult, err := migrator.Redo(cmd.Context())
			if err != nil {
				return err
			}

			if downResult != nil {
				cmd.Printf("ROLLED BACK  %s (%s)\n", downResult.Source.Path, downResult.Duration)
			}

			if upResult != nil {
				cmd.Printf("OK           %s (%s)\n", upResult.Source.Path, upResult.Duration)
			}

			return nil
		},
	}
}
