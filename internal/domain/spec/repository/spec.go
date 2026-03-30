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
	"time"
)

// TaskSpec describes a task scoped to a repository partition.
// Domain declares it, application multiplies it per partition via Pack().
//
// Domain is a horse with blinders — it declares work and its partition need,
// not retry strategy, not lifecycle phase.
// Retry is handled by Runner's Baseline (configured by Scheduler).
// Phase is determined by the registry DSL (Preflight marker in Pack).
//
// Examples:
//   - Validator: Resource="repository-validator", Interval=OneShot, Work=validate
//   - IssuePoller: Resource="issue-poller", Interval=10s, Work=poll
type TaskSpec struct {
	Resource string
	Interval time.Duration // spec.OneShot | >0 | 0=panic
	Work     func(ctx context.Context, partition Partition) error
}
