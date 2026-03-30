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
)

// OneShot marks a task as one-shot (execute once, not on interval).
// Use in TaskSpec.Interval instead of zero — zero panics at plan time.
const OneShot time.Duration = -1

// Phase determines when a task runs in the scheduler lifecycle.
type Phase int

const (
	// PhaseWork is the default phase. Tasks run after all preflights complete.
	// Zero-value — safe default.
	PhaseWork Phase = iota

	// PhasePreflight tasks run first. Environment may be incomplete
	// (e.g. repository DB row may not exist yet).
	PhasePreflight
)

// GlobalTaskSpec describes a task not scoped to any partition.
// Compile-time guard: Work takes only context — cannot be passed to partition Pack().
//
// Examples:
//   - IssueExplainer: Resource="issue-explainer", Interval=2s, Work=poll queue
type GlobalTaskSpec struct {
	Resource string
	Interval time.Duration // OneShot | >0 | 0=panic
	Work     func(ctx context.Context) error
}

// Task is a ready-to-schedule task produced by application layer.
// Planner receives these, Scheduler executes them.
type Task struct {
	Name     string
	Interval time.Duration
	Phase    Phase
	Work     func(context.Context) error
}
