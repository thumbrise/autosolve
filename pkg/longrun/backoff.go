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
//
// Formula: delay = Initial * Multiplier^attempt
//
// Example with Initial=1s, Multiplier=2.0, Max=30s:
//
//	attempt 0: 1s * 2^0 = 1s
//	attempt 1: 1s * 2^1 = 2s
//	attempt 2: 1s * 2^2 = 4s
//	attempt 3: 1s * 2^3 = 8s
//	...
//	attempt 5: 1s * 2^5 = 32s → capped at 30s
type BackoffConfig struct {
	// Initial is the delay before the first retry (attempt 0).
	// Must be > 0. Validated at Task construction time.
	Initial time.Duration
	// Max caps the computed delay. 0 means no cap (use with caution —
	// combined with UnlimitedRetries, delay grows until overflow).
	Max time.Duration
	// Multiplier scales the delay on each consecutive attempt.
	// Must be > 0. Validated at Task construction time.
	// 1.0 = constant delay, 2.0 = classic exponential, 1.5 = gentle growth.
	Multiplier float64
}

// DefaultBackoff returns a sensible default backoff configuration.
//
// Configured as Initial=1s, Max=30s, Multiplier=2.0
//
// Perfect for 5 retries
func DefaultBackoff() BackoffConfig {
	return BackoffConfig{
		Initial:    1 * time.Second,
		Max:        30 * time.Second,
		Multiplier: 2.0,
	}
}

// Duration returns the backoff duration for the given 0-based attempt index.
//
// When Max > 0, the result is capped at Max.
// When Max is 0 (no cap) and the computed duration overflows (e.g. after
// thousands of attempts with UnlimitedRetries), Duration clamps to math.MaxInt64.
func (b *BackoffConfig) Duration(attempt int) time.Duration {
	d := float64(b.Initial) * math.Pow(b.Multiplier, float64(attempt))
	if b.Max > 0 && d > float64(b.Max) {
		d = float64(b.Max)
	}

	if math.IsInf(d, 0) || math.IsNaN(d) || d > float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
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
