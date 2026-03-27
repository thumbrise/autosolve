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

import "time"

// RateLimitError is a domain-visible rate limit error.
// It carries RetryAfter so the caller can sleep precisely until the limit resets.
// Does NOT depend on go-github — domain imports only this type.
//
// Implements apierr.Retryable, apierr.WaitHinted, apierr.ServicePressure.
type RateLimitError struct {
	RetryAfter time.Duration
	Err        error
}

func (e *RateLimitError) Error() string               { return e.Err.Error() }
func (e *RateLimitError) Unwrap() error               { return e.Err }
func (e *RateLimitError) Retryable() bool             { return true }
func (e *RateLimitError) WaitDuration() time.Duration { return e.RetryAfter }
func (e *RateLimitError) ServicePressure() bool       { return true }

// ServerError represents a server-side failure (HTTP 5xx).
// The server acknowledged the request but failed to process it.
//
// Implements apierr.Retryable.
type ServerError struct {
	StatusCode int
	Err        error
}

func (e *ServerError) Error() string   { return e.Err.Error() }
func (e *ServerError) Unwrap() error   { return e.Err }
func (e *ServerError) Retryable() bool { return true }
