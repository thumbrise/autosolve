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

// Package apierr defines error interfaces for API integration convention.
//
// Any infrastructure client (GitHub, Jira, Slack) returns errors implementing
// these interfaces. The application-level classifier checks them to determine
// retry category and wait strategy.
//
// This is a shared contract — both infrastructure and application layers
// depend on it. Domain layer does not.
package apierr

import "time"

// Retryable indicates the operation can be retried.
// Infrastructure returns Retryable() == true for errors where a retry
// has a reasonable chance of success (e.g. HTTP 5xx, temporary failures).
type Retryable interface {
	error
	Retryable() bool
}

// WaitHinted indicates the error carries an explicit wait duration.
// The caller should sleep for WaitDuration() before retrying instead of
// using exponential backoff (e.g. Retry-After header on HTTP 429).
type WaitHinted interface {
	error
	WaitDuration() time.Duration
}

// ServicePressure indicates the remote service is under pressure.
// Used to select a gentler retry policy (longer backoff, longer max interval)
// compared to transport-level errors (e.g. HTTP 429, HTTP 503).
type ServicePressure interface {
	error
	ServicePressure() bool
}
