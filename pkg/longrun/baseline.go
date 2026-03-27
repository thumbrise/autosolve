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

import "time"

// ErrorCategory classifies an error for baseline policy selection.
type ErrorCategory int

const (
	// CategoryUnknown means the error was not recognized by any classifier.
	// If Baseline.Degraded is set — retry with degraded policy.
	// If Baseline.Degraded is nil — permanent error.
	CategoryUnknown ErrorCategory = iota

	// CategoryNode indicates a transport-level failure (TCP, DNS, TLS, timeout).
	// The request never reached the server or the connection was interrupted.
	// Retry aggressively — the network will recover.
	CategoryNode

	// CategoryService indicates the remote service is under pressure
	// (rate limit, 5xx, maintenance). Retry gently — don't kick them
	// while they're down.
	CategoryService
)

// ErrorClass is the result of error classification.
// Returned by ClassifierFunc to tell handleFailure which category the error
// belongs to and optionally how long to wait before retrying.
type ErrorClass struct {
	// Category determines which baseline policy to use.
	Category ErrorCategory

	// WaitDuration, when > 0, overrides the backoff calculation.
	// The task sleeps exactly this duration instead of using
	// policy.Backoff.Duration(attempt).
	// Typical source: Retry-After header on HTTP 429.
	WaitDuration time.Duration
}

// ClassifierFunc inspects an error and returns its classification.
// Return nil if the error is not recognized — the next classification
// step will handle it.
//
// ClassifierFunc must be safe to call concurrently from multiple goroutines.
type ClassifierFunc func(err error) *ErrorClass

// Policy defines retry behavior for a single error category.
type Policy struct {
	// Retries limits consecutive retry attempts.
	//   0 (zero-value) → unlimited retries (baseline default).
	//  >0 → exact retry count.
	Retries int

	// Backoff controls exponential backoff between retries.
	Backoff BackoffConfig
}

// Baseline is a set of policies that Runner silently applies to every task.
// Tasks don't know about baseline — it's configured once on Runner.
//
// Classification pipeline in handleFailure:
//
//	[1] Built-in transport classify (net.OpError, timeout → Node)
//	[2] User classifier via Classify (apierr interfaces → Service)
//	[3] Not classified → Unknown ->
//	    Unknown + Degraded != nil → retry with Degraded policy (LOUD log)
//	    Unknown + Degraded == nil → permanent error
type Baseline struct {
	// Node policy for transport-level errors (TCP, DNS, TLS, timeout).
	// Aggressive retry — network will recover.
	Node Policy

	// Service policy for service-pressure errors (rate limit, 5xx).
	// Gentle retry — don't overload the remote service.
	Service Policy

	// Degraded policy for unknown/unclassified errors.
	// nil → unknown errors are permanent (crash). Use for preflights.
	// non-nil → retry with loud ERROR logging. Use for workers.
	Degraded *Policy

	// Classify is the application-level classifier.
	// Called after built-in transport classification.
	// nil = no application classification, only transport + degraded.
	Classify ClassifierFunc
}

// isZero reports whether b is the zero-value Baseline (no policies configured).
func (b *Baseline) isZero() bool {
	return b.Node == (Policy{}) && b.Service == (Policy{}) && b.Degraded == nil && b.Classify == nil
}

// Backoff is a convenience constructor for BackoffConfig with sensible defaults.
// Multiplier defaults to 2.0 (classic exponential backoff).
//
// Example:
//
//	longrun.Backoff(2*time.Second, 2*time.Minute)
//	// → BackoffConfig{Initial: 2s, Max: 2m, Multiplier: 2.0}
func Backoff(initial, maxCap time.Duration) BackoffConfig {
	return BackoffConfig{
		Initial:    initial,
		Max:        maxCap,
		Multiplier: 2.0,
	}
}
