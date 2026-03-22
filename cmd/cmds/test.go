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
	"log/slog"

	"github.com/spf13/cobra"
)

// Test command for new commands registration architecture changes.
//
// Will be removed after adds new real production commands.
//
// Added as proof-of-concept
type Test struct {
	*cobra.Command
}

func NewTest(logger *slog.Logger) *Test {
	c := &cobra.Command{
		Use:   "test",
		Short: "Test command for new commands registration architecture changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.DebugContext(cmd.Context(), "Test Output Via Injected Logger")

			return nil
		},
	}

	return &Test{c}
}

// TestSubTree command for new commands registration architecture changes.
//
// Will be removed after adds new real production commands.
//
// Added as proof-of-concept
type TestSubTree struct {
	*cobra.Command
}

func NewTestSubTree(logger *slog.Logger) *TestSubTree {
	c := &cobra.Command{
		Use:   "subtree",
		Short: "Sub Tree Test command for new commands sub tree registration architecture changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.DebugContext(cmd.Context(), "Test Sub Tree Output Via Injected Logger")

			return nil
		},
	}

	return &TestSubTree{c}
}
