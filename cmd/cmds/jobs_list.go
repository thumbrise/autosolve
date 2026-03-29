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
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/thumbrise/autosolve/internal/infrastructure/queue"
)

// JobsList lists all pending messages in the goqite queue.
type JobsList struct {
	*cobra.Command
}

func NewJobsList(db *sql.DB) *JobsList {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show all pending messages in the queue",
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, err := db.QueryContext(cmd.Context(),
				"SELECT id, created, updated, body, timeout, received, priority FROM goqite WHERE queue = ?",
				queue.QueueName,
			)
			if err != nil {
				return fmt.Errorf("query goqite: %w", err)
			}

			defer func() { _ = rows.Close() }()

			t := table.NewWriter()
			t.SetOutputMirror(cmd.OutOrStdout())
			t.AppendHeader(table.Row{"ID", "Type", "Repository ID", "Issue ID", "Received", "Created"})

			for rows.Next() {
				var (
					id       string
					created  string
					updated  string
					body     []byte
					timeout  string
					received int64
					priority int64
				)

				if err := rows.Scan(&id, &created, &updated, &body, &timeout, &received, &priority); err != nil {
					return fmt.Errorf("scan row: %w", err)
				}

				var msg queue.JobMessage
				if err := json.Unmarshal(body, &msg); err != nil {
					msg = queue.JobMessage{Type: "(unmarshal error)"}
				}

				t.AppendRow(table.Row{id, msg.Type, msg.RepositoryID, msg.IssueID, received, created})
			}

			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate rows: %w", err)
			}

			t.Render()

			return nil
		},
	}

	return &JobsList{cmd}
}
