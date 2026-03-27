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

package limit

import (
	"golang.org/x/time/rate"

	"github.com/thumbrise/autosolve/internal/config"
)

// MinIntervalThrottler wraps rate.Limiter to avoid binding a generic stdlib type in Wire.
// Burst is always 1 — requests are serialized with a guaranteed minimum interval.
type MinIntervalThrottler struct {
	*rate.Limiter
}

// NewMinIntervalThrottler creates a MinIntervalThrottler from config.
// Rate is computed as 1/MinInterval.
func NewMinIntervalThrottler(cfg *config.Github) *MinIntervalThrottler {
	r := rate.Every(cfg.RateLimit.MinInterval)

	return &MinIntervalThrottler{rate.NewLimiter(r, 1)}
}
