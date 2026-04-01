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

package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/thumbrise/autosolve/internal/application/schedule"
	"github.com/thumbrise/autosolve/internal/application/schedule/sdsl"
)

func TestNewPlan_SplitsByPhase(t *testing.T) {
	noop := func(context.Context) error { return nil }

	jobs := []sdsl.Job{
		sdsl.SetupJob("validate", noop),
		sdsl.WorkerJob("poller", 10*time.Second, noop),
		sdsl.SetupJob("migrate", noop),
		sdsl.WorkerJob("relay", 5*time.Second, noop),
	}

	plan := schedule.NewPlan(jobs)

	if len(plan.Setup) != 2 {
		t.Fatalf("expected 2 setup jobs, got %d", len(plan.Setup))
	}

	if len(plan.Work) != 2 {
		t.Fatalf("expected 2 work jobs, got %d", len(plan.Work))
	}
}

func TestNewPlan_EmptyJobs(t *testing.T) {
	plan := schedule.NewPlan(nil)

	if len(plan.Setup) != 0 {
		t.Fatalf("expected 0 setup jobs, got %d", len(plan.Setup))
	}

	if len(plan.Work) != 0 {
		t.Fatalf("expected 0 work jobs, got %d", len(plan.Work))
	}
}

func TestNewPlan_PanicsOnDuplicateName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate job name")
		}
	}()

	noop := func(context.Context) error { return nil }

	schedule.NewPlan([]sdsl.Job{
		sdsl.SetupJob("validate", noop),
		sdsl.SetupJob("validate", noop),
	})
}

func TestNewPlan_PanicsOnNoPhasePrefix(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on missing phase prefix")
		}
	}()

	noop := func(context.Context) error { return nil }

	schedule.NewPlan([]sdsl.Job{
		{Name: "no-prefix", Work: noop},
	})
}

func TestNewPlan_PanicsOnUnknownPhase(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on unknown phase")
		}
	}()

	noop := func(context.Context) error { return nil }

	schedule.NewPlan([]sdsl.Job{
		{Name: "unknown:job", Work: noop},
	})
}
