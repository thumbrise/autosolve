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
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thumbrise/autosolve/internal/infrastructure/queue"
)

// JobsShow displays a single goqite message with full payload.
type JobsShow struct {
	*cobra.Command
}

func NewJobsShow(db *sql.DB) *JobsShow {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show one message with full payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			var (
				msgID    string
				created  string
				updated  string
				body     []byte
				timeout  string
				received int64
				priority int64
			)

			err := db.QueryRowContext(cmd.Context(),
				"SELECT id, created, updated, body, timeout, received, priority FROM goqite WHERE id = ? AND queue = ?",
				id, queue.QueueName,
			).Scan(&msgID, &created, &updated, &body, &timeout, &received, &priority)
			if errors.Is(err, sql.ErrNoRows) {
				cmd.PrintErrln("message not found")

				return nil
			}

			if err != nil {
				return fmt.Errorf("query goqite: %w", err)
			}

			var pretty []byte

			if p, err := json.MarshalIndent(json.RawMessage(body), "", "  "); err == nil {
				pretty = p
			} else {
				pretty = body
			}
			if err != nil {
				return fmt.Errorf("json marshal: %w", err)
			}

			cmd.Printf("ID:        %s\n", msgID)
			cmd.Printf("Created:   %s\n", created)
			cmd.Printf("Updated:   %s\n", updated)
			cmd.Printf("Timeout:   %s\n", timeout)
			cmd.Printf("Received:  %d\n", received)
			cmd.Printf("Priority:  %d\n", priority)
			cmd.Printf("Body:\n%s\n", pretty)

			return nil
		},
	}

	return &JobsShow{cmd}
}
