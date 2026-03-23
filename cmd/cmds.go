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
	cmds.NewTest,
	cmds.NewTestSubTree,
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
	testCMD *cmds.Test,
	testSubTree *cmds.TestSubTree,
) []*cobra.Command {
	testCMD.AddCommand(testSubTree.Command)

	return []*cobra.Command{
		scheduleCMD.Command,
		testCMD.Command,
	}
}
