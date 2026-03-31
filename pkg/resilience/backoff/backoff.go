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

// Package backoff provides backoff functions for resilience patterns.
//
// A [Func] computes the delay before the next retry given a 0-based attempt
// index. Pure function — no side effects, no context, no IO.
package backoff

import (
	"math"
	"time"
)

// Func computes the delay before the next retry given a 0-based attempt
// index. Pure function — no side effects, no context, no IO.
//
// The package provides common constructors ([Exponential], [Constant]), but any
// function with this signature works — jitter, decorrelated, adaptive, etc.
//
// Example (custom jittered backoff):
//
//	func jittered(attempt int) time.Duration {
//	    base := time.Second * time.Duration(1<<attempt)
//	    return base + time.Duration(rand.Int63n(int64(base/2)))
//	}
type Func func(attempt int) time.Duration

// Exponential returns a Func with classic exponential growth.
// Multiplier is 2.0. Delay is capped at maxCap.
//
// Formula: delay = initial * 2^attempt, capped at maxCap.
//
// Example:
//
//	backoff.Exponential(2*time.Second, 2*time.Minute)
//	// attempt 0: 2s, attempt 1: 4s, attempt 2: 8s, ..., capped at 2m
func Exponential(initial, maxCap time.Duration) Func {
	return ExponentialWith(initial, maxCap, 2.0)
}

// ExponentialWith returns a Func with configurable multiplier.
//
// Formula: delay = initial * multiplier^attempt, capped at maxCap.
//
// Example:
//
//	backoff.ExponentialWith(1*time.Second, 30*time.Second, 1.5)
//	// attempt 0: 1s, attempt 1: 1.5s, attempt 2: 2.25s, ...
func ExponentialWith(initial, maxCap time.Duration, multiplier float64) Func {
	return func(attempt int) time.Duration {
		d := float64(initial) * math.Pow(multiplier, float64(attempt))
		if maxCap > 0 && d > float64(maxCap) {
			d = float64(maxCap)
		}

		if math.IsInf(d, 0) || math.IsNaN(d) || d > float64(math.MaxInt64) {
			return time.Duration(math.MaxInt64)
		}

		return time.Duration(d)
	}
}

// Constant returns a Func that always returns the same delay.
// Useful for retry-after scenarios or testing.
//
// Example:
//
//	backoff.Constant(5*time.Second)
func Constant(d time.Duration) Func {
	return func(attempt int) time.Duration {
		return d
	}
}

// Default returns a sensible default Func.
//
// Configured as Exponential(1s, 30s) — classic doubling, capped at 30s.
// Perfect for 5 retries: 1s, 2s, 4s, 8s, 16s.
func Default() Func {
	return Exponential(1*time.Second, 30*time.Second)
}
