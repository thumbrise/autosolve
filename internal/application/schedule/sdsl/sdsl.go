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

// Package sdsl provides the schedule DSL toolkit for job providers.
//
// Providers use [SetupJob], [WorkerJob], and [Join] to build jobs.
// Phase constants [PhaseSetup] and [PhaseWork] define execution order.
//
// This package has no dependency on the schedule package itself,
// breaking the import cycle between schedule and its provider sub-packages.
package sdsl

import (
	"context"
	"time"
)

// Well-known phase names. Used as job name prefixes and strategy map keys.
const (
	PhaseSetup = "setup"
	PhaseWork  = "work"
)

// SetupJob creates a one-shot Job for the setup phase.
// Setup jobs run before workers. Use for validation, migrations, etc.
//
//	sdsl.SetupJob("validate-repos", validateFn)
func SetupJob(name string, work func(context.Context) error) Job {
	return Job{
		Name: PhaseSetup + ":" + name,
		Work: work,
	}
}

// WorkerJob creates a recurring Job for the work phase.
// Worker jobs run on a ticker after all setup jobs complete.
//
//	sdsl.WorkerJob("poll-issues", 10*time.Second, pollFn)
func WorkerJob(name string, interval time.Duration, work func(context.Context) error) Job {
	if interval <= 0 {
		panic("sdsl.WorkerJob: interval must be > 0 for " + name)
	}

	return Job{
		Name:     PhaseWork + ":" + name,
		Interval: interval,
		Work:     work,
	}
}

// Join concatenates job slices from multiple providers.
//
//	sdsl.Join(
//	    repoProvider.Jobs(),
//	    globalProvider.Jobs(),
//	)
func Join(groups ...[]Job) []Job {
	var out []Job

	for _, g := range groups {
		out = append(out, g...)
	}

	return out
}

// Job is a ready-to-schedule unit of work produced by the application layer.
// Planner groups jobs into phases, Scheduler executes them.
type Job struct {
	Name     string
	Interval time.Duration
	Work     func(context.Context) error
}
