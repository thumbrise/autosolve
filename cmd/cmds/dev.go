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

// dev.go is a throwaway PoC dev dashboard — NOT production code.
// Single-file HTTP server with embedded HTML, raw SQL, inline SSE.
// Will be replaced by proper architecture when the dispatch pipeline matures.
// See: https://github.com/thumbrise/autosolve/issues/152

package cmds

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/thumbrise/autosolve/internal/infrastructure/dal/sqlcgen"
	"github.com/thumbrise/autosolve/internal/infrastructure/ollama"
)

const devServerReadHeaderTimeout = 10 * time.Second

//go:embed devui
var devUI embed.FS

type Dev struct {
	*cobra.Command
}

func NewDev(db *sql.DB, queries *sqlcgen.Queries, ollamaClient *ollama.Client, logger *slog.Logger) *Dev {
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Start dev dashboard for AI prompt playground",
		RunE: func(cmd *cobra.Command, args []string) error {
			port, _ := cmd.Flags().GetInt("port")
			addr := fmt.Sprintf(":%d", port)

			s := &devServer{db: db, queries: queries, ollama: ollamaClient, logger: logger}

			mux := http.NewServeMux()
			mux.HandleFunc("/", s.handleIndex)
			mux.HandleFunc("/api/issues", s.handleIssues)
			mux.HandleFunc("/api/oneshot", s.handleOneShot)
			mux.HandleFunc("/api/events", s.handleEvents)
			mux.HandleFunc("/api/replay", s.handleReplay)

			srv := &http.Server{
				Addr:              addr,
				Handler:           mux,
				ReadHeaderTimeout: devServerReadHeaderTimeout,
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			go func() {
				<-ctx.Done()

				shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				defer cancel()

				logger.InfoContext(shutdownCtx, "shutting down dev server")
				_ = srv.Shutdown(shutdownCtx)
			}()

			logger.InfoContext(cmd.Context(), "dev dashboard starting", slog.String("addr", "http://localhost"+addr))
			cmd.Printf("Dev dashboard → http://localhost%s\n", addr)

			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}

			return nil
		},
	}

	cmd.Flags().IntP("port", "p", 8080, "HTTP port")

	return &Dev{cmd}
}

type devServer struct {
	db      *sql.DB
	queries *sqlcgen.Queries
	ollama  *ollama.Client
	logger  *slog.Logger
}

func (s *devServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	f, err := devUI.Open("devui/index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)

		return
	}

	defer func() { _ = f.Close() }()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.Copy(w, f)
}

type issueEvent struct {
	Type     string `json:"type"`
	EventID  int64  `json:"eventId"`
	IssueNum int64  `json:"issueNum"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	Prompt   string `json:"prompt"`
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

func (s *devServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	events, err := s.queries.PendingOutboxEventsAll(ctx, s.db, sqlcgen.PendingOutboxEventsAllParams{
		Topic: "issues:synced",
		Limit: 100,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)

		return
	}

	prompt := r.URL.Query().Get("prompt")
	if prompt == "" {
		prompt = "Classify this GitHub issue. Suggest priority (critical/high/medium/low) and component."
	}

	// Send total count first.
	_, _ = fmt.Fprintf(w, "data: {\"type\":\"total\",\"count\":%d}\n\n", len(events))

	flusher.Flush()

	for _, ev := range events {
		if ctx.Err() != nil {
			return
		}

		s.processOutboxEvent(ctx, w, flusher, sqlcgen.PendingOutboxEventsRow(ev), prompt)
	}

	_, _ = fmt.Fprintf(w, "data: {\"type\":\"complete\"}\n\n")

	flusher.Flush()
}

func (s *devServer) processOutboxEvent(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, ev sqlcgen.PendingOutboxEventsRow, prompt string) {
	issue, err := s.queries.GetIssueByRepoAndNumber(ctx, s.db, sqlcgen.GetIssueByRepoAndNumberParams{
		RepositoryID: ev.RepositoryID,
		Number:       ev.ResourceID,
	})
	if err != nil {
		s.sendSSE(w, flusher, issueEvent{Type: "error", EventID: ev.ID, IssueNum: ev.ResourceID, Error: err.Error()})

		return
	}

	fullPrompt := fmt.Sprintf("%s\n\nTitle: %s\nBody: %s", prompt, issue.Title, issue.Body)

	s.sendSSE(w, flusher, issueEvent{
		Type: "processing", EventID: ev.ID, IssueNum: issue.Number,
		Title: issue.Title, Body: issue.Body, Prompt: fullPrompt,
	})

	aiResponse, err := s.ollama.Generate(ctx, fullPrompt)
	if err != nil {
		s.sendSSE(w, flusher, issueEvent{Type: "error", EventID: ev.ID, IssueNum: issue.Number, Title: issue.Title, Error: err.Error()})

		return
	}

	_ = s.queries.AckOutboxEvent(ctx, s.db, ev.ID)

	s.sendSSE(w, flusher, issueEvent{
		Type: "done", EventID: ev.ID, IssueNum: issue.Number,
		Title: issue.Title, Body: issue.Body, Prompt: fullPrompt, Response: aiResponse,
	})
}

func (s *devServer) handleReplay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)

		return
	}

	const (
		resetAll     = "UPDATE outbox_events SET processed_at = NULL WHERE processed_at IS NOT NULL AND topic = 'issues:synced'"
		resetLimited = "UPDATE outbox_events SET processed_at = NULL WHERE id IN (SELECT id FROM outbox_events WHERE processed_at IS NOT NULL AND topic = 'issues:synced' ORDER BY created_at ASC LIMIT ?)"
	)

	query := resetAll
	args := []any{}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			query = resetLimited

			args = append(args, limit)
		}
	}

	res, err := s.db.ExecContext(r.Context(), query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	count, _ := res.RowsAffected()

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]int64{"reset": count})
}

func (s *devServer) handleIssues(w http.ResponseWriter, r *http.Request) {
	issues, err := s.queries.ListIssues(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, issues)
}

func (s *devServer) handleOneShot(w http.ResponseWriter, r *http.Request) {
	repoID, _ := strconv.ParseInt(r.URL.Query().Get("repo"), 10, 64)
	number, _ := strconv.ParseInt(r.URL.Query().Get("number"), 10, 64)
	prompt := r.URL.Query().Get("prompt")

	if repoID == 0 || number == 0 {
		http.Error(w, "repo and number required", http.StatusBadRequest)

		return
	}

	if prompt == "" {
		prompt = "Classify this GitHub issue. Suggest priority (critical/high/medium/low) and component."
	}

	issue, err := s.queries.GetIssueByRepoAndNumber(r.Context(), s.db, sqlcgen.GetIssueByRepoAndNumberParams{
		RepositoryID: repoID, Number: number,
	})
	if err != nil {
		http.Error(w, "issue not found: "+err.Error(), http.StatusNotFound)

		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)

		return
	}

	s.generateAndStream(r.Context(), w, flusher, issue, prompt)

	_, _ = fmt.Fprintf(w, "data: {\"type\":\"complete\"}\n\n")

	flusher.Flush()
}

func (s *devServer) generateAndStream(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, issue sqlcgen.GetIssueByRepoAndNumberRow, prompt string) {
	fullPrompt := fmt.Sprintf("%s\n\nTitle: %s\nBody: %s", prompt, issue.Title, issue.Body)

	s.sendSSE(w, flusher, issueEvent{
		Type: "processing", IssueNum: issue.Number,
		Title: issue.Title, Body: issue.Body, Prompt: fullPrompt,
	})

	aiResponse, err := s.ollama.Generate(ctx, fullPrompt)
	if err != nil {
		s.sendSSE(w, flusher, issueEvent{Type: "error", IssueNum: issue.Number, Title: issue.Title, Error: err.Error()})

		return
	}

	s.sendSSE(w, flusher, issueEvent{
		Type: "done", IssueNum: issue.Number,
		Title: issue.Title, Body: issue.Body, Prompt: fullPrompt, Response: aiResponse,
	})
}

func (s *devServer) sendSSE(w http.ResponseWriter, flusher http.Flusher, ev issueEvent) {
	data, err := json.Marshal(ev)
	if err != nil {
		s.logger.Error("failed to marshal SSE event", slog.Any("error", err))

		return
	}

	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)

	flusher.Flush()
}

func writeJSON(w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
