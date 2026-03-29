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

package cmd

import (
	"github.com/google/wire"
	"github.com/spf13/cobra"

	"github.com/thumbrise/autosolve/cmd/cmds"
)

var Bindings = wire.NewSet(
	NewRoot,
	NewCommands,
	cmds.NewSchedule,
	cmds.NewMigrate,
	cmds.NewMigrateUp,
	cmds.NewMigrateUpFresh,
	cmds.NewMigrateDown,
	cmds.NewMigrateStatus,
	cmds.NewMigrateCreate,
	cmds.NewMigrateRedo,
	cmds.NewTest,
	cmds.NewTestSubTree,
	cmds.NewOutbox,
	cmds.NewOutboxReplay,
	cmds.NewJobs,
	cmds.NewJobsList,
	cmds.NewJobsShow,
	cmds.NewDev,
)

// NewCommands is the central CLI command registry.
//
// Wire instantiates all command providers listed as parameters,
// then this function assembles them into []*cobra.Command for Root.
//
// Subcommand tree is built here — attach children to parents via AddCommand.
// Only root-level commands are returned; Root receives and registers them.
//
// Example with namespace:
//
//	func NewCommands(
//	    schedule *cmds.Schedule,
//	    configCmd *cmds.ConfigCmd,
//	    configSet *cmds.ConfigSet,
//	) []*cobra.Command {
//	    configCmd.AddCommand(configSet.Command) // build tree here
//
//	    return []*cobra.Command{               // return only root commands
//	        schedule.Command,
//	        configCmd.Command,
//	    }
//	}
//
// Adding a new command:
//  1. Create constructor in cmd/cmds/ (e.g. NewVersion(...deps) *Version)
//  2. Add provider to Bindings and parameter + line here
//
// Wire guarantees compile-time safety in both directions:
//   - Provider in Bindings but missing here → "unused provider"
//   - Parameter here but missing in Bindings → "no provider for"
func NewCommands(
	scheduleCMD *cmds.Schedule,
	migrateCMD *cmds.Migrate,
	migrateUp *cmds.MigrateUp,
	migrateUpFresh *cmds.MigrateUpFresh,
	migrateDown *cmds.MigrateDown,
	migrateStatus *cmds.MigrateStatus,
	migrateCreate *cmds.MigrateCreate,
	migrateRedo *cmds.MigrateRedo,
	testCMD *cmds.Test,
	testSubTree *cmds.TestSubTree,
	outboxCMD *cmds.Outbox,
	outboxReplay *cmds.OutboxReplay,
	jobsCMD *cmds.Jobs,
	jobsList *cmds.JobsList,
	jobsShow *cmds.JobsShow,
	devCMD *cmds.Dev,
) []*cobra.Command {
	migrateCMD.AddCommand(
		migrateUp.Command,
		migrateUpFresh.Command,
		migrateDown.Command,
		migrateStatus.Command,
		migrateCreate.Command,
		migrateRedo.Command,
	)

	testCMD.AddCommand(testSubTree.Command)

	outboxCMD.AddCommand(outboxReplay.Command)

	jobsCMD.AddCommand(jobsList.Command, jobsShow.Command)

	return []*cobra.Command{
		scheduleCMD.Command,
		migrateCMD.Command,
		testCMD.Command,
		outboxCMD.Command,
		jobsCMD.Command,
		devCMD.Command,
	}
}
