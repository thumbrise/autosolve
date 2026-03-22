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
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

var ErrMigrationFailed = errors.New("migration failed")

type Migrator struct {
	db     *gorm.DB
	models []interface{}
	logger *slog.Logger
}

func NewMigrator(db *gorm.DB, logger *slog.Logger, models []interface{}) *Migrator {
	return &Migrator{db: db, logger: logger, models: models}
}

func (m *Migrator) Migrate(ctx context.Context) error {
	err := m.db.WithContext(ctx).AutoMigrate(m.models...)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrMigrationFailed, err)
	}

	m.logger.InfoContext(ctx, "database migrate success")

	return nil
}
