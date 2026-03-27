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

package spec

import (
	"context"
	"time"

	"github.com/thumbrise/autosolve/internal/domain/spec/tenants"
)

// PreflightSpec describes a one-shot task that must complete before workers start.
// Defined by domain, consumed by Planner.
//
// Domain declares which errors are transient via Transients.
// Planner decides how to retry them (backoff, max retries).
//
// Task naming convention: preflight:{Resource}:{owner}/{name}
// Examples:
//   - preflight:repository-validator:thumbrise/autosolve
type PreflightSpec struct {
	Resource   string
	Transients []error
	Work       func(ctx context.Context, tenant tenants.RepositoryTenant) error
}

// WorkerSpec describes a long-running interval task.
// Defined by domain, consumed by Planner.
//
// Domain declares which errors are transient via Transients.
// Planner decides how to retry them (backoff, max retries).
//
// Task naming convention: worker:{Resource}:{owner}/{name}
// Examples:
//   - worker:issue-poller:thumbrise/autosolve
//   - worker:comment-poller:thumbrise/otelext
type WorkerSpec struct {
	Resource   string
	Interval   time.Duration
	Transients []error
	Work       func(ctx context.Context, tenant tenants.RepositoryTenant) error
}
