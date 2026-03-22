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
	"log/slog"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	loggergorm "gorm.io/gorm/logger"

	"github.com/thumbrise/autosolve/internal/config"
)

func NewGormDB(logger *slog.Logger, cfg *config.Database, cfgApp *config.App) (*gorm.DB, error) {
	options := SQLiteOptions{
		Path: cfg.SQLitePath,
		Pragma: map[string]string{
			"journal_mode": "WAL",
			"foreign_keys": "1",
			"busy_timeout": "5000",
		},
	}
	dial := sqlite.Open(options.DSN())

	logLevel := loggergorm.Silent
	if cfgApp.Debug {
		logLevel = loggergorm.Info
	}

	cfgGorm := &gorm.Config{
		DefaultTransactionTimeout: 5 * time.Second,
		DefaultContextTimeout:     10 * time.Second,
		Logger: loggergorm.NewSlogLogger(logger, loggergorm.Config{
			SlowThreshold:             200 * time.Millisecond,
			Colorful:                  false,
			IgnoreRecordNotFoundError: false,
			ParameterizedQueries:      false,
			LogLevel:                  logLevel,
		}),
	}

	db, err := gorm.Open(dial, cfgGorm)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// SQLite works best with a single connection to avoid SQLITE_BUSY errors.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	// Keep connection alive forever — no reason to recycle with a single SQLite connection.
	sqlDB.SetConnMaxLifetime(0)

	return db, nil
}
