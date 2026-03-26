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

package github

import (
	"net/http"

	"golang.org/x/time/rate"

	"github.com/thumbrise/autosolve/internal/config"
)

// RateLimiter wraps rate.Limiter to avoid binding a generic stdlib type in Wire.
// Burst is always 1 — requests are serialized with a guaranteed minimum interval.
type RateLimiter struct {
	limiter *rate.Limiter
}

// NewRateLimiter creates a RateLimiter from config.
// Rate is computed as 1/MinInterval.
func NewRateLimiter(cfg *config.Github) *RateLimiter {
	r := rate.Every(cfg.RateLimit.MinInterval)

	return &RateLimiter{limiter: rate.NewLimiter(r, 1)}
}

// rateLimitedTransport is an http.RoundTripper that waits for the rate limiter
// before every outgoing HTTP request. This makes rate limiting transparent
// to the GitHub client — no method signatures need to change.
type rateLimitedTransport struct {
	base    http.RoundTripper
	limiter *RateLimiter
}

func newRateLimitedTransport(base http.RoundTripper, limiter *RateLimiter) *rateLimitedTransport {
	return &rateLimitedTransport{base: base, limiter: limiter}
}

func (t *rateLimitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.limiter.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}

	return t.base.RoundTrip(req)
}
