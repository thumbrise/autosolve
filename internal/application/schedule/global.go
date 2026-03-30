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
	"fmt"

	"github.com/thumbrise/autosolve/internal/domain/spec"
)

// globalTasks wraps GlobalTaskSpecs into Tasks. No partition, no multiplication.
//
// Panics on zero Interval (must use spec.OneShot).
func globalTasks(specs ...spec.GlobalTaskSpec) []spec.Task {
	tasks := make([]spec.Task, 0, len(specs))

	for _, s := range specs {
		if s.Interval == 0 {
			panic(fmt.Sprintf("schedule: global spec %q has zero Interval — use spec.OneShot", s.Resource))
		}

		if s.Interval < 0 {
			//nolint:godox // global preflight DSL deferred — no current use case.
			// TODO: add GlobalPreflight() marker when needed, similar to Preflight() for repository tasks.
			panic(fmt.Sprintf("schedule: global spec %q has OneShot interval but no GlobalPreflight() marker — not yet supported", s.Resource))
		}

		tasks = append(tasks, spec.Task{
			Name:     "worker:" + s.Resource,
			Interval: s.Interval,
			Phase:    spec.PhaseWork,
			Work:     s.Work,
		})
	}

	return tasks
}

// join concatenates task slices. Registry helper.
func join(groups ...[]spec.Task) []spec.Task {
	var out []spec.Task

	for _, g := range groups {
		out = append(out, g...)
	}

	return out
}
