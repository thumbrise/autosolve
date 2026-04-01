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

	"github.com/thumbrise/autosolve/internal/application/schedule/globals"
	"github.com/thumbrise/autosolve/internal/application/schedule/repos"
	"github.com/thumbrise/autosolve/internal/application/schedule/sdsl"
)

var Bindings = wire.NewSet(
	NewScheduler,
	NewPlan,
	NewResilienceClient,
	NewJobs,

	repos.Bindings,
	globals.Bindings,
)

// NewJobs is the declarative job registry.
// Read it as a manifest: which providers contribute jobs.
//
// Each provider owns its domain tasks, intervals, and DSL.
// Adding a new provider: one parameter + one line.
//
// Pattern mirrors cmd.NewCommands — providers register themselves,
// registry assembles.
func NewJobs(
	repoProvider *repos.Provider,
	globalProvider *globals.Provider,
) []sdsl.Job {
	return sdsl.Join(
		repoProvider.Jobs(),
		globalProvider.Jobs(),
	)
}
