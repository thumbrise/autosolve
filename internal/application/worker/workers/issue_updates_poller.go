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

package workers

import (
	"log/slog"
	"net"

	"github.com/thumbrise/autosolve/internal/config"
	"github.com/thumbrise/autosolve/internal/domain/issue"
	"github.com/thumbrise/autosolve/pkg/longrun"
)

type IssueUpdatesPoller struct {
	cfg    *config.Github
	logger *slog.Logger
	parser *issue.Parser
}

func NewIssueUpdatesPoller(cfg *config.Github, logger *slog.Logger, parser *issue.Parser) *IssueUpdatesPoller {
	return &IssueUpdatesPoller{cfg: cfg, logger: logger, parser: parser}
}

// Task returns a longrun.Task that polls issues on the configured interval.
func (iup *IssueUpdatesPoller) Task() *longrun.Task {
	transientRules := longrun.TransientGroup(
		5,
		longrun.DefaultBackoff(),
		iup.transients()...,
	)

	return longrun.NewIntervalTask(
		"workers.IssueUpdatesPoller",
		iup.cfg.Issues.ParseInterval,
		iup.parser.Run,
		transientRules,
		longrun.WithLogger(iup.logger),
	)
}

func (iup *IssueUpdatesPoller) transients() []error {
	// TODO: specify work specific transients
	return []error{
		(*net.OpError)(nil),
	}
}
