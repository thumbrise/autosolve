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
// Predefined categories cover common network integration scenarios.
// Users can define custom categories for domain-specific classification:
//
//	const CategoryDatabase longrun.ErrorCategory = 10
type ErrorCategory int

const (
	// CategoryUnknown means the error was not recognized by any classifier.
	// If Baseline.Default is set — retry with default policy.
	// If Baseline.Default is nil — permanent error.
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
	// The task sleeps exactly this duration instead of calling
	// policy.Backoff(attempt).
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

	// Backoff computes the delay before the next retry.
	// Use Exponential, Constant, or any custom BackoffFunc.
	Backoff BackoffFunc
}

// Baseline is a set of policies that Runner silently applies to every task.
// Tasks don't know about baseline — it's configured once on Runner.
//
// Policies maps error categories to retry policies. Use predefined categories
// (CategoryNode, CategoryService) or define your own.
//
// Classification pipeline in handleFailure:
//
//	[1] Built-in transport classify (net.OpError, timeout → Node)
//	[2] User classifier via Classify (apierr interfaces → Service)
//	[3] Not classified → Unknown ->
//	    Unknown + Default != nil → retry with Default policy (LOUD log)
//	    Unknown + Default == nil → permanent error
type Baseline struct {
	// Policies maps error categories to their retry policies.
	// Use predefined categories (CategoryNode, CategoryService) or define custom ones.
	Policies map[ErrorCategory]Policy

	// Default policy for errors not matching any category in Policies.
	// nil → unknown errors are permanent (crash). Use for preflights.
	// non-nil → retry with loud ERROR logging. Use for workers.
	Default *Policy

	// Classify is the application-level classifier.
	// Called after built-in transport classification.
	// nil = no application classification, only transport + default.
	Classify ClassifierFunc
}

// NewBaseline creates a Baseline with Node and Service policies.
// Default is nil — unknown errors are permanent.
//
// Example:
//
//	longrun.NewBaseline(
//	    longrun.Policy{Backoff: longrun.Exponential(2*time.Second, 2*time.Minute)},
//	    longrun.Policy{Backoff: longrun.Exponential(5*time.Second, 5*time.Minute)},
//	    myClassifier,
//	)
func NewBaseline(node, service Policy, classify ClassifierFunc) Baseline {
	return Baseline{
		Policies: map[ErrorCategory]Policy{
			CategoryNode:    node,
			CategoryService: service,
		},
		Classify: classify,
	}
}

// NewBaselineDegraded creates a Baseline with Node, Service, and Default policies.
// Unknown errors retry with Default policy instead of crashing.
//
// Example:
//
//	longrun.NewBaselineDegraded(
//	    longrun.Policy{Backoff: longrun.Exponential(2*time.Second, 2*time.Minute)},
//	    longrun.Policy{Backoff: longrun.Exponential(5*time.Second, 5*time.Minute)},
//	    longrun.Policy{Backoff: longrun.Exponential(30*time.Second, 5*time.Minute)},
//	    myClassifier,
//	)
func NewBaselineDegraded(node, service, defaultPolicy Policy, classify ClassifierFunc) Baseline {
	b := NewBaseline(node, service, classify)
	b.Default = &defaultPolicy

	return b
}

// isZero reports whether b is the zero-value Baseline (no policies configured).
func (b *Baseline) isZero() bool {
	return len(b.Policies) == 0 && b.Default == nil && b.Classify == nil
}
