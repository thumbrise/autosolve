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
	"math"
	"time"
)

const (
	// UnlimitedRetries disables the retry limit — the task retries forever
	// (until a permanent error or context cancellation).
	// Use with caution: set this explicitly to opt in.
	UnlimitedRetries = -1

	// DefaultMaxRetries is used when MaxRetries is 0 (zero-value).
	DefaultMaxRetries = 3
)

// BackoffConfig controls exponential backoff between retry attempts.
type BackoffConfig struct {
	Initial    time.Duration
	Max        time.Duration
	Multiplier float64
}

// DefaultBackoff returns a sensible default backoff configuration.
func DefaultBackoff() BackoffConfig {
	return BackoffConfig{
		Initial:    1 * time.Second,
		Max:        30 * time.Second,
		Multiplier: 2.0,
	}
}

// Duration returns the backoff duration for the given 0-based attempt index.
func (b *BackoffConfig) Duration(attempt int) time.Duration {
	d := float64(b.Initial) * math.Pow(b.Multiplier, float64(attempt))
	if b.Max > 0 && d > float64(b.Max) {
		d = float64(b.Max)
	}

	return time.Duration(d)
}

// Wait blocks for the backoff duration of the given attempt, or until ctx is cancelled.
func (b *BackoffConfig) Wait(ctx context.Context, attempt int) error {
	timer := time.NewTimer(b.Duration(attempt))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
