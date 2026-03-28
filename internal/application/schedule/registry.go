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

import (
	"github.com/google/wire"

	"github.com/thumbrise/autosolve/internal/domain/spec/preflights"
	"github.com/thumbrise/autosolve/internal/domain/spec/workers"
)

var Bindings = wire.NewSet(
	NewScheduler,
	NewPlanner,
	NewPreflights,
	NewWorkers,

	preflights.NewRepositoryValidator,
	workers.NewIssuePoller,
	workers.NewOutboxRelay,
)

// NewPreflights registers all preflight tasks.
// Add new preflights here when extending the system.
func NewPreflights(
	repoValidator *preflights.RepositoryValidator,
) []Preflight {
	return []Preflight{
		repoValidator,
	}
}

// NewWorkers registers all worker tasks.
// Add new workers here when extending the system.
func NewWorkers(
	issuePoller *workers.IssuePoller,
	outboxRelay *workers.OutboxRelay,
) []Worker {
	return []Worker{
		issuePoller,
		outboxRelay,
	}
}
