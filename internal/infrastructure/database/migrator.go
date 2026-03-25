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

package database

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// migrationsDir is the on-disk path to the migrations directory.
// Used only by Create to write new migration files during development.
const migrationsDir = "internal/infrastructure/database/migrations"

var ErrMigrationFailed = errors.New("migration failed")

// Migrator runs goose migrations against the application database.
//
// Runtime operations (Up, Down, Status, Redo) use the embedded migration files
// compiled into the binary. Create writes new files to disk and is intended
// for development only.
type Migrator struct {
	provider *goose.Provider
	logger   *slog.Logger
}

func NewMigrator(db *gorm.DB, logger *slog.Logger) (*Migrator, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("%w: get sql.DB: %w", ErrMigrationFailed, err)
	}

	migrations, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("%w: sub fs: %w", ErrMigrationFailed, err)
	}

	provider, err := goose.NewProvider(goose.DialectSQLite3, sqlDB, migrations)
	if err != nil {
		return nil, fmt.Errorf("%w: create provider: %w", ErrMigrationFailed, err)
	}

	return &Migrator{provider: provider, logger: logger}, nil
}

// Up applies pending migrations. If steps <= 0, applies all pending migrations.
// Otherwise applies exactly that many steps.
func (m *Migrator) Up(ctx context.Context, steps int) ([]*goose.MigrationResult, error) {
	if steps <= 0 {
		results, err := m.provider.Up(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: up: %w", ErrMigrationFailed, err)
		}

		m.logApplied(ctx, results)

		return results, nil
	}

	results := make([]*goose.MigrationResult, 0, steps)

	for range steps {
		r, err := m.provider.UpByOne(ctx)
		if err != nil {
			return results, fmt.Errorf("%w: up: %w", ErrMigrationFailed, err)
		}

		if r == nil {
			break
		}

		results = append(results, r)
	}

	m.logApplied(ctx, results)

	return results, nil
}

// Down rolls back applied migrations. Steps must be > 0, or use DownAll.
func (m *Migrator) Down(ctx context.Context, steps int) ([]*goose.MigrationResult, error) {
	results := make([]*goose.MigrationResult, 0, steps)

	for range steps {
		r, err := m.provider.Down(ctx)
		if err != nil {
			return results, fmt.Errorf("%w: down: %w", ErrMigrationFailed, err)
		}

		if r == nil {
			break
		}

		results = append(results, r)
	}

	m.logRolledBack(ctx, results)

	return results, nil
}

// DownAll rolls back all applied migrations.
func (m *Migrator) DownAll(ctx context.Context) ([]*goose.MigrationResult, error) {
	results, err := m.provider.DownTo(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: down all: %w", ErrMigrationFailed, err)
	}

	m.logRolledBack(ctx, results)

	return results, nil
}

// Status returns the status of all known migrations.
func (m *Migrator) Status(ctx context.Context) ([]*goose.MigrationStatus, error) {
	results, err := m.provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: status: %w", ErrMigrationFailed, err)
	}

	return results, nil
}

// Redo rolls back the last migration and re-applies only that one migration.
func (m *Migrator) Redo(ctx context.Context) ([]*goose.MigrationResult, []*goose.MigrationResult, error) {
	downResults, err := m.Down(ctx, 1)
	if err != nil {
		return nil, nil, err
	}

	upResults, err := m.Up(ctx, 1)
	if err != nil {
		return downResults, nil, err
	}

	return downResults, upResults, nil
}

// Fresh drops all tables and re-applies all migrations from scratch.
// Development-only operation — equivalent to Laravel's migrate:fresh.
func (m *Migrator) Fresh(ctx context.Context) ([]*goose.MigrationResult, error) {
	if _, err := m.DownAll(ctx); err != nil {
		return nil, err
	}

	return m.Up(ctx, 0)
}

func (m *Migrator) logApplied(ctx context.Context, results []*goose.MigrationResult) {
	for _, r := range results {
		m.logger.InfoContext(ctx, "migration applied",
			slog.String("file", r.Source.Path),
			slog.String("duration", r.Duration.String()),
		)
	}
}

func (m *Migrator) logRolledBack(ctx context.Context, results []*goose.MigrationResult) {
	for _, r := range results {
		m.logger.InfoContext(ctx, "migration rolled back",
			slog.String("file", r.Source.Path),
			slog.String("duration", r.Duration.String()),
		)
	}
}

var sqlMigrationTemplate = template.Must(template.New("goose.sql-migration").Parse(
	`-- +goose Up

-- +goose Down
`))

// Create writes a new empty SQL migration file to disk. Development-only operation.
func (m *Migrator) Create(name string) (string, error) {
	if err := os.MkdirAll(migrationsDir, 0o750); err != nil {
		return "", fmt.Errorf("%w: ensure dir: %w", ErrMigrationFailed, err)
	}

	filename := fmt.Sprintf("%s_%s.sql", time.Now().Format("20060102150405"), name)
	path := filepath.Join(migrationsDir, filename)

	f, err := os.Create(path) //nolint:gosec // path is built from const migrationsDir + developer CLI input
	if err != nil {
		return "", fmt.Errorf("%w: create file: %w", ErrMigrationFailed, err)
	}
	defer f.Close()

	if err := sqlMigrationTemplate.Execute(f, nil); err != nil {
		return "", fmt.Errorf("%w: write template: %w", ErrMigrationFailed, err)
	}

	return path, nil
}
