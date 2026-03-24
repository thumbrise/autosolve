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

// Package httperr classifies HTTP errors into transient and permanent
// based on objective HTTP/REST semantics.
//
// Transient errors are those where retrying the same request may succeed:
// network failures, timeouts, rate limits, server errors (5xx).
//
// Permanent errors are those where retrying is pointless:
// bad request (400), unauthorized (401), not found (404), etc.
package httperr

import "errors"

// Sentinel errors for transient HTTP failures.
// Use with longrun.TransientRule or errors.Is for classification.
var (
	// ErrNetwork indicates a transport-level failure: TCP, DNS, TLS.
	// The request never reached the server or the connection was interrupted.
	ErrNetwork = errors.New("httperr: network error")
	// ErrTimeout indicates the request or response exceeded a deadline.
	// Covers both client-side timeouts and HTTP 408 Request Timeout.
	ErrTimeout = errors.New("httperr: timeout")
	// ErrRateLimit indicates the server rejected the request due to
	// rate limiting (HTTP 429 Too Many Requests).
	ErrRateLimit = errors.New("httperr: rate limit")
	// ErrServerError indicates a server-side failure (HTTP 5xx).
	// The server acknowledged the request but failed to process it.
	ErrServerError = errors.New("httperr: server error")
)
