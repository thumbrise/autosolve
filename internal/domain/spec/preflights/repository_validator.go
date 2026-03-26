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

package preflights

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/thumbrise/autosolve/internal/domain/spec"
	"github.com/thumbrise/autosolve/internal/domain/spec/tenants"
	"github.com/thumbrise/autosolve/internal/infrastructure/dal/repositories"
	githubinfra "github.com/thumbrise/autosolve/internal/infrastructure/github"
)

var ErrValidateRepository = errors.New("validate repository")

// RepositoryValidator checks that a repository exists and is accessible via GitHub API,
// then upserts it into the local database.
// Implements application.Preflight via TaskSpec().
type RepositoryValidator struct {
	githubClient *githubinfra.Client
	repoRepo     *repositories.RepositoryRepository
	logger       *slog.Logger
}

func NewRepositoryValidator(githubClient *githubinfra.Client, repoRepo *repositories.RepositoryRepository, logger *slog.Logger) *RepositoryValidator {
	return &RepositoryValidator{githubClient: githubClient, repoRepo: repoRepo, logger: logger}
}

// TaskSpec returns a PreflightSpec that validates and upserts a repository.
// Any error is permanent — if a repo is inaccessible, the app should not start.
func (v *RepositoryValidator) TaskSpec() spec.PreflightSpec {
	return spec.PreflightSpec{
		Resource:   "repository-validator",
		Transients: nil, // no retries — all errors are permanent
		Work:       v.validate,
	}
}

func (v *RepositoryValidator) validate(ctx context.Context, tenant tenants.RepositoryTenant) error {
	v.logger.InfoContext(ctx, "validating repository",
		slog.String("owner", tenant.Owner),
		slog.String("name", tenant.Name),
	)

	_, err := v.githubClient.GetRepository(ctx, tenant.Owner, tenant.Name)
	if err != nil {
		return fmt.Errorf("%w: %s/%s: %w", ErrValidateRepository, tenant.Owner, tenant.Name, err)
	}

	_, err = v.repoRepo.Upsert(ctx, tenant.Owner, tenant.Name)
	if err != nil {
		return fmt.Errorf("%w: upsert %s/%s: %w", ErrValidateRepository, tenant.Owner, tenant.Name, err)
	}

	v.logger.InfoContext(ctx, "repository validated",
		slog.String("owner", tenant.Owner),
		slog.String("name", tenant.Name),
	)

	return nil
}
