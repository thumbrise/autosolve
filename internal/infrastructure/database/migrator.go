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
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/thumbrise/autosolve/internal/config"
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
//
// Methods that apply migrations return []*goose.MigrationResult and error.
// On partial failure goose returns a *goose.PartialError containing both
// the successfully applied migrations and the failed one.
// Migrator preserves this contract — callers can use errors.As to extract
// partial results when needed.
type Migrator struct {
	provider *goose.Provider
	db       *sql.DB
	dbPath   string
}

func NewMigrator(db *sql.DB, cfg *config.Database) (*Migrator, error) {
	migrations, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("%w: sub fs: %w", ErrMigrationFailed, err)
	}

	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations)
	if err != nil {
		return nil, fmt.Errorf("%w: create provider: %w", ErrMigrationFailed, err)
	}

	return &Migrator{provider: provider, db: db, dbPath: cfg.SQLitePath}, nil
}

// Up applies pending migrations. If steps <= 0, applies all pending migrations.
// Otherwise applies exactly that many steps.
//
// On partial failure the error wraps *goose.PartialError with applied results.
func (m *Migrator) Up(ctx context.Context, steps int) ([]*goose.MigrationResult, error) {
	if steps <= 0 {
		results, err := m.provider.Up(ctx)
		if err != nil {
			return results, fmt.Errorf("%w: up: %w", ErrMigrationFailed, err)
		}

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

	return results, nil
}

// DownAll rolls back all applied migrations.
func (m *Migrator) DownAll(ctx context.Context) ([]*goose.MigrationResult, error) {
	results, err := m.provider.DownTo(ctx, 0)
	if err != nil {
		return results, fmt.Errorf("%w: down all: %w", ErrMigrationFailed, err)
	}

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
		return downResults, nil, err
	}

	upResults, err := m.Up(ctx, 1)
	if err != nil {
		return downResults, upResults, err
	}

	return downResults, upResults, nil
}

// Fresh deletes the SQLite database file and re-applies all migrations from scratch.
// Development-only operation — equivalent to Laravel's migrate:fresh.
//
// This avoids FK constraint issues during rollback by removing the file entirely.
// The database connection is closed, the file is deleted, a new connection is opened
// via NewDB, and the goose provider is recreated before running Up.
//
// After Fresh returns, the old *sql.DB (held by other components via Wire) is closed.
// This is safe because Fresh is a CLI-only operation — the process exits after it.
func (m *Migrator) Fresh(ctx context.Context) ([]*goose.MigrationResult, []*goose.MigrationResult, error) {
	// Close the current connection so SQLite releases the file.
	if err := m.db.Close(); err != nil {
		return nil, nil, fmt.Errorf("%w: close db: %w", ErrMigrationFailed, err)
	}

	// Remove the database file (and WAL/SHM sidecars if present).
	if err := os.Remove(m.dbPath); err != nil && !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("%w: remove db file: %w", ErrMigrationFailed, err)
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(m.dbPath + suffix)
	}

	// Reopen via NewDB to avoid duplicating connection setup logic.
	db, err := NewDB(ctx, &config.Database{SQLitePath: m.dbPath})
	if err != nil {
		return nil, nil, fmt.Errorf("%w: reopen db: %w", ErrMigrationFailed, err)
	}

	m.db = db

	migrations, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		return nil, nil, fmt.Errorf("%w: sub fs: %w", ErrMigrationFailed, err)
	}

	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: recreate provider: %w", ErrMigrationFailed, err)
	}

	m.provider = provider

	upResults, err := m.Up(ctx, 0)

	return nil, upResults, err
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
