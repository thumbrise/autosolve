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
	"errors"
	"fmt"
	"time"

	"github.com/google/go-github/v84/github"
)

// mapError converts GitHub API errors into domain-visible error types
// that implement apierr interfaces (Retryable, WaitHinted, ServicePressure).
//
// Classification order:
//  1. GitHub-specific rate limit types (*github.RateLimitError, *github.AbuseRateLimitError)
//     → RateLimitError (implements Retryable + WaitHinted + ServicePressure).
//     GitHub returns 403 for rate limits, not 429.
//  2. Other GitHub HTTP errors (*github.ErrorResponse) with 5xx status
//     → ServerError (implements Retryable).
//  3. Everything else is returned as-is.
//     Transport errors (net.OpError, timeout) are handled by longrun built-in classifier.
//
// The original error is always preserved in the chain via Unwrap.
func (p *Client) mapError(err error) error {
	if err == nil {
		return nil
	}

	// GitHub-Case: rate limit as 403, not 429.
	// Wrapped into our RateLimitError so domain never imports go-github.
	if rl, ok := errors.AsType[*github.RateLimitError](err); ok {
		return &RateLimitError{
			RetryAfter: time.Until(rl.Rate.Reset.Time),
			Err:        fmt.Errorf("rate limit: %w", err),
		}
	}

	if abuse, ok := errors.AsType[*github.AbuseRateLimitError](err); ok {
		var retryAfter time.Duration
		if abuse.RetryAfter != nil {
			retryAfter = *abuse.RetryAfter
		}

		return &RateLimitError{
			RetryAfter: retryAfter,
			Err:        fmt.Errorf("abuse rate limit: %w", err),
		}
	}

	// Server errors (5xx) — retryable.
	if ghErr, ok := errors.AsType[*github.ErrorResponse](err); ok && ghErr.Response != nil {
		if ghErr.Response.StatusCode >= 500 {
			return &ServerError{
				StatusCode: ghErr.Response.StatusCode,
				Err:        fmt.Errorf("server error %d: %w", ghErr.Response.StatusCode, err),
			}
		}
	}

	// Everything else returned as-is.
	// Transport and unknown errors are classified by the caller.
	return err
}
