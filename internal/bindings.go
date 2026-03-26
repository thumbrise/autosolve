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

package internal

import (
	"github.com/google/wire"

	"github.com/thumbrise/autosolve/internal/application"
	"github.com/thumbrise/autosolve/internal/config"
	"github.com/thumbrise/autosolve/internal/domain/issue"
	"github.com/thumbrise/autosolve/internal/domain/repository"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/repositories"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/sqlcgen"
	"github.com/thumbrise/autosolve/internal/infrastructure/database"
	"github.com/thumbrise/autosolve/internal/infrastructure/github"
)

var Bindings = wire.NewSet(
	config.NewGithub,
	config.NewDatabase,

	database.NewDB,
	database.NewMigrator,
	sqlcgen.New,

	github.NewRateLimiter,
	github.NewGithubClient,
	github.NewClient,

	application.NewScheduler,
	application.NewPlanner,
	application.NewPreflights,
	application.NewWorkers,

	repository.NewValidator,
	issue.NewParser,

	repositories.NewIssueRepository,
	repositories.NewRepositoryRepository,
)
