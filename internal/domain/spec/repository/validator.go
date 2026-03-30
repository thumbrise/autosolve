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

package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/thumbrise/autosolve/internal/domain"
	"github.com/thumbrise/autosolve/internal/domain/spec"
	githubinfra "github.com/thumbrise/autosolve/internal/infrastructure/github"
)

var ErrValidateRepository = errors.New("validate repository")

// Validator checks that a repository exists and is accessible via GitHub API,
// then upserts it into the local database.
type Validator struct {
	githubClient *githubinfra.Client
	repoStore    domain.RepositoryStore
	logger       *slog.Logger
}

func NewValidator(githubClient *githubinfra.Client, repoStore domain.RepositoryStore, logger *slog.Logger) *Validator {
	return &Validator{githubClient: githubClient, repoStore: repoStore, logger: logger}
}

// TaskSpec returns a TaskSpec. Phase is set by the registry via Preflight().
func (v *Validator) TaskSpec() TaskSpec {
	return TaskSpec{
		Resource: "repository-validator",
		Interval: spec.OneShot,
		Work:     v.validate,
	}
}

func (v *Validator) validate(ctx context.Context, partition Partition) error {
	v.logger.InfoContext(ctx, "validating repository",
		slog.String("owner", partition.Owner),
		slog.String("name", partition.Name),
	)

	_, err := v.githubClient.GetRepository(ctx, partition.Owner, partition.Name)
	if err != nil {
		return fmt.Errorf("%w: %s/%s: %w", ErrValidateRepository, partition.Owner, partition.Name, err)
	}

	_, err = v.repoStore.Upsert(ctx, partition.Owner, partition.Name)
	if err != nil {
		return fmt.Errorf("%w: upsert %s/%s: %w", ErrValidateRepository, partition.Owner, partition.Name, err)
	}

	v.logger.InfoContext(ctx, "repository validated",
		slog.String("owner", partition.Owner),
		slog.String("name", partition.Name),
	)

	return nil
}
