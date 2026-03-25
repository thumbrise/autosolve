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

// Up applies all pending migrations and returns individual results.
func (m *Migrator) Up(ctx context.Context) ([]*goose.MigrationResult, error) {
	results, err := m.provider.Up(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: up: %w", ErrMigrationFailed, err)
	}

	for _, r := range results {
		m.logger.InfoContext(ctx, "migration applied",
			slog.String("file", r.Source.Path),
			slog.String("duration", r.Duration.String()),
		)
	}

	return results, nil
}

// Down rolls back the last applied migration.
func (m *Migrator) Down(ctx context.Context) (*goose.MigrationResult, error) {
	result, err := m.provider.Down(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: down: %w", ErrMigrationFailed, err)
	}

	if result != nil {
		m.logger.InfoContext(ctx, "migration rolled back",
			slog.String("file", result.Source.Path),
			slog.String("duration", result.Duration.String()),
		)
	}

	return result, nil
}

// Status returns the status of all known migrations.
func (m *Migrator) Status(ctx context.Context) ([]*goose.MigrationStatus, error) {
	results, err := m.provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: status: %w", ErrMigrationFailed, err)
	}

	return results, nil
}

// Redo rolls back the last migration and re-applies it.
func (m *Migrator) Redo(ctx context.Context) (*goose.MigrationResult, *goose.MigrationResult, error) {
	down, err := m.provider.Down(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: redo down: %w", ErrMigrationFailed, err)
	}

	results, err := m.provider.Up(ctx)
	if err != nil {
		return down, nil, fmt.Errorf("%w: redo up: %w", ErrMigrationFailed, err)
	}

	var up *goose.MigrationResult
	if len(results) > 0 {
		up = results[0]
	}

	return down, up, nil
}

// Fresh drops all tables and re-applies all migrations from scratch.
// Development-only operation — equivalent to Laravel's migrate:fresh.
func (m *Migrator) Fresh(ctx context.Context) ([]*goose.MigrationResult, error) {
	downResults, err := m.provider.DownTo(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: fresh down: %w", ErrMigrationFailed, err)
	}

	for _, r := range downResults {
		m.logger.InfoContext(ctx, "migration rolled back",
			slog.String("file", r.Source.Path),
			slog.String("duration", r.Duration.String()),
		)
	}

	upResults, err := m.provider.Up(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: fresh up: %w", ErrMigrationFailed, err)
	}

	for _, r := range upResults {
		m.logger.InfoContext(ctx, "migration applied",
			slog.String("file", r.Source.Path),
			slog.String("duration", r.Duration.String()),
		)
	}

	return upResults, nil
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

	f, err := os.Create(path) //nolint:gosec // path is built from const migrationsDir + sanitized name
	if err != nil {
		return "", fmt.Errorf("%w: create file: %w", ErrMigrationFailed, err)
	}
	defer f.Close()

	if err := sqlMigrationTemplate.Execute(f, nil); err != nil {
		return "", fmt.Errorf("%w: write template: %w", ErrMigrationFailed, err)
	}

	return path, nil
}
