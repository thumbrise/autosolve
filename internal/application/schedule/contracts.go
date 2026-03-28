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

package schedule

import "github.com/thumbrise/autosolve/internal/domain/spec"

// Preflight is a one-shot task that must complete before workers start.
// Implementations return a PreflightSpec describing the work to be done per tenant.
//
// Task naming convention: preflight:{Resource}:{owner}/{name}
// Examples:
//   - preflight:repository-validator:thumbrise/autosolve
type Preflight interface {
	TaskSpec() spec.PreflightSpec
}

// Worker is a long-running interval task.
// Implementations return a WorkerSpec describing the work to be done per tenant.
//
// Task naming convention: worker:{Resource}:{owner}/{name}
// Examples:
//   - worker:issue-poller:thumbrise/autosolve
//   - worker:comment-poller:thumbrise/otelext
type Worker interface {
	TaskSpec() spec.WorkerSpec
}
