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

package schedule

import (
	"time"

	"github.com/thumbrise/resilience"
	"github.com/thumbrise/resilience/backoff"
	rsotel "github.com/thumbrise/resilience/otel"
	"github.com/thumbrise/resilience/retry"
)

// NewResilienceClient creates the application-wide resilience Client with OTEL plugin.
func NewResilienceClient() *resilience.Client {
	return resilience.NewClient(rsotel.Plugin())
}

// strictRetryOptions returns retry options for setup phase.
// Known transport and service errors are retried.
// Unregistered errors crash the phase — fix your config.
func strictRetryOptions() []resilience.Option {
	return []resilience.Option{
		retry.OnFunc(isNodeError, retry.Unlimited, backoff.Exponential(2*time.Second, 2*time.Minute), "node"),
		retry.OnFunc(isServiceError, retry.Unlimited, backoff.Exponential(5*time.Second, 5*time.Minute), "service",
			retry.WithWaitHint(serviceWaitHint),
		),
	}
}

// resilientRetryOptions returns retry options for work phase.
// Same as strict, plus a catch-all for unregistered errors —
// the scheduler must keep running, even if it doesn't recognize the error.
func resilientRetryOptions() []resilience.Option {
	return []resilience.Option{
		retry.OnFunc(isNodeError, retry.Unlimited, backoff.Exponential(2*time.Second, 2*time.Minute), "node"),
		retry.OnFunc(isServiceError, retry.Unlimited, backoff.Exponential(5*time.Second, 5*time.Minute), "service",
			retry.WithWaitHint(serviceWaitHint),
		),
		retry.OnFunc(always, retry.Unlimited, backoff.Exponential(30*time.Second, 5*time.Minute), "unregistered"),
	}
}
