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

package cmds

import (
	"github.com/spf13/cobra"

	"github.com/thumbrise/autosolve/internal/application"
	"github.com/thumbrise/autosolve/internal/bootstrap/contracts"
)

type Schedule struct {
	*cobra.Command
}

func NewSchedule(r contracts.RootCMD, scheduler *application.Scheduler) *Schedule {
	c := &cobra.Command{
		Use:   "schedule",
		Short: "Poll github and dispatch AI tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			return scheduler.Run(cmd.Context())
		},
	}
	r.AddCommand(c)

	return &Schedule{c}
}
