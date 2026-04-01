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

// Package globals is the global (non-partitioned) job provider for the scheduler.
package globals

import (
	"time"

	"github.com/google/wire"

	"github.com/thumbrise/autosolve/internal/application/schedule/sdsl"
	domglobal "github.com/thumbrise/autosolve/internal/domain/tasks/global"
)

// Bindings provides Wire bindings for the global job provider.
var Bindings = wire.NewSet(
	NewProvider,

	domglobal.NewIssueExplainer,
)

// Provider builds global (non-partitioned) jobs.
type Provider struct {
	explainer *domglobal.IssueExplainer
}

// NewProvider creates a global job provider.
func NewProvider(explainer *domglobal.IssueExplainer) *Provider {
	return &Provider{explainer: explainer}
}

// Jobs returns ready-to-schedule global jobs.
func (p *Provider) Jobs() []sdsl.Job {
	return []sdsl.Job{
		sdsl.WorkerJob("issue-explainer", 2*time.Second, p.explainer.Run),
	}
}
