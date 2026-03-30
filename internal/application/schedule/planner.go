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

// Planner receives ready-to-schedule Tasks from the registry and splits them by Phase.
// It validates inputs (paranoid) but does not multiply or transform — that's done by providers.
type Planner struct {
	tasks []spec.Task
}

// NewPlanner creates a Planner from Tasks provided by the registry.
// Panics on duplicate Task.Name — name is an operationally important identifier.
func NewPlanner(tasks []spec.Task) *Planner {
	seen := make(map[string]struct{}, len(tasks))

	for _, t := range tasks {
		if _, exists := seen[t.Name]; exists {
			panic(fmt.Sprintf("planner: duplicate task name %q — check registry for double registration", t.Name))
		}

		seen[t.Name] = struct{}{}
	}

	return &Planner{tasks: tasks}
}

// Preflights returns tasks in PhasePreflight.
func (p *Planner) Preflights() []spec.Task {
	var out []spec.Task

	for _, t := range p.tasks {
		if t.Phase == spec.PhasePreflight {
			out = append(out, t)
		}
	}

	return out
}

// Workers returns tasks in PhaseWork.
func (p *Planner) Workers() []spec.Task {
	var out []spec.Task

	for _, t := range p.tasks {
		if t.Phase == spec.PhaseWork {
			out = append(out, t)
		}
	}

	return out
}
