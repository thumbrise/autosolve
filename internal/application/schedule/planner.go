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

	"github.com/thumbrise/autosolve/internal/application/schedule/sdsl"
)

// Plan holds jobs grouped by phase. Setup runs first, then Work.
// Scheduler knows the order — Plan is just data.
type Plan struct {
	Setup []sdsl.Job
	Work  []sdsl.Job
}

// NewPlan creates a Plan from jobs. Jobs are split by phase prefix in their name.
// Panics on duplicate job names.
func NewPlan(jobs []sdsl.Job) *Plan {
	seen := make(map[string]struct{}, len(jobs))

	var plan Plan

	for _, j := range jobs {
		if _, exists := seen[j.Name]; exists {
			panic(fmt.Sprintf("planner: duplicate job name %q — check registry for double registration", j.Name))
		}

		seen[j.Name] = struct{}{}

		phase := extractPhase(j.Name)

		switch phase {
		case sdsl.PhaseSetup:
			plan.Setup = append(plan.Setup, j)
		case sdsl.PhaseWork:
			plan.Work = append(plan.Work, j)
		default:
			panic(fmt.Sprintf("planner: unknown phase %q in job %q", phase, j.Name))
		}
	}

	return &plan
}

// extractPhase extracts the phase name from a job name.
// Job names follow the convention "phase:resource:partition".
func extractPhase(name string) string {
	for i, c := range name {
		if c == ':' {
			return name[:i]
		}
	}

	panic(fmt.Sprintf("planner: job name %q has no phase prefix (expected 'phase:...')", name))
}
