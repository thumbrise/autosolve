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

// outbox.go contains dev-only CLI utilities for the outbox table.
// Throwaway PoC tooling — will be replaced or removed. See #152.

package cmds

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
)

// Outbox is a parent command for outbox dev utilities.
type Outbox struct {
	*cobra.Command
}

func NewOutbox() *Outbox {
	return &Outbox{&cobra.Command{
		Use:   "outbox",
		Short: "Outbox dev utilities",
	}}
}

// OutboxReplay resets processed_at so the explainer re-processes events.
type OutboxReplay struct {
	*cobra.Command
}

func NewOutboxReplay(db *sql.DB, logger *slog.Logger) *OutboxReplay {
	cmd := &cobra.Command{
		Use:   "replay",
		Short: "Reset outbox events so they are re-processed by workers",
		RunE: func(cmd *cobra.Command, args []string) error {
			topic, _ := cmd.Flags().GetString("topic")

			query := "UPDATE outbox_events SET processed_at = NULL WHERE processed_at IS NOT NULL"
			qArgs := []any{}

			if topic != "" {
				query += " AND topic = ?"

				qArgs = append(qArgs, topic)
			}

			res, err := db.ExecContext(cmd.Context(), query, qArgs...)
			if err != nil {
				return fmt.Errorf("reset outbox: %w", err)
			}

			count, _ := res.RowsAffected()

			logger.InfoContext(cmd.Context(), "outbox events reset",
				slog.Int64("count", count),
				slog.String("topic", topic),
			)

			cmd.Printf("reset %d outbox event(s)\n", count)

			return nil
		},
	}

	cmd.Flags().StringP("topic", "t", "", "Filter by topic (e.g. issues:synced)")

	return &OutboxReplay{cmd}
}
