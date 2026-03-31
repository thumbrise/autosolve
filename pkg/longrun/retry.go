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

package longrun

import (
	"context"
	"log/slog"
	"time"
)

// retryParams holds the parameters for a single retry decision.
// Used by doRetry to avoid duplicating the increment → budget → wait → log → sleep
// skeleton across ruleFailureHandler and baselineFailureHandler.
type retryParams struct {
	key          string        // AttemptStore key, e.g. "rule:fetch" or "baseline:node"
	maxRetries   int           // resolved value: UnlimitedRetries (-1) = no limit, >0 = exact budget
	backoff      BackoffFunc   // computes delay from 0-based attempt index
	waitOverride time.Duration // if > 0, used instead of backoff (e.g. ErrorClass.WaitDuration)
	logLevel     slog.Level    // log level for the retry message
	logMsg       string        // log message
}

// doRetry executes the common retry algorithm:
//
//	increment attempt → check budget → compute wait duration → log → sleep → return nil.
//
// Returns the original err when the retry budget is exhausted, nil otherwise.
func doRetry(ctx context.Context, err error, p retryParams, attempts AttemptStore, logger *slog.Logger) error {
	attempt := attempts.Increment(p.key)

	if p.maxRetries != UnlimitedRetries && attempt >= p.maxRetries {
		logger.ErrorContext(ctx, "max retries reached",
			slog.Any("error", err),
			slog.Int("max_retries", p.maxRetries),
		)

		return err
	}

	var waitDur time.Duration

	if p.waitOverride > 0 {
		waitDur = p.waitOverride
	} else {
		waitDur = p.backoff(attempt)
	}

	logger.Log(ctx, p.logLevel, p.logMsg,
		slog.Int("attempt", attempt+1),
		slog.Any("error", err),
		slog.Any("backoff", waitDur),
	)

	sleepCtx(ctx, waitDur)

	return nil
}
