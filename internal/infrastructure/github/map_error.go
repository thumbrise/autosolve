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

	"github.com/thumbrise/autosolve/pkg/httperr"
)

// mapError classifies a GitHub API error into a transient or permanent sentinel.
//
// Classification order:
//  1. GitHub-specific rate limit types (*github.RateLimitError, *github.AbuseRateLimitError)
//     are mapped to httperr.ErrRateLimit directly, because GitHub returns 403 for rate limits
//     which would otherwise be classified as permanent by generic HTTP classification.
//  2. Other GitHub HTTP errors (*github.ErrorResponse) are classified by status code
//     via httperr.ClassifyStatus (408 → timeout, 429 → rate limit, 5xx → server error).
//  3. Transport-level errors (net.OpError, url.Error, DNS) are classified via httperr.Classify.
//  4. Everything else is returned as-is (permanent).
//
// The original error is always preserved in the chain — callers can use errors.As
// to access GitHub-specific fields (e.g. RateLimitError.Rate.Reset).
func (p *Client) mapError(err error) error {
	if err == nil {
		return nil
	}

	// GitHub-Case: rate limit as 403, not 429.
	// Wrapped into our RateLimitError so domain never imports go-github.
	if rl, ok := errors.AsType[*github.RateLimitError](err); ok {
		return &RateLimitError{
			RetryAfter: time.Until(rl.Rate.Reset.Time),
			Err:        fmt.Errorf("%w: %w", httperr.ErrRateLimit, err),
		}
	}

	if abuse, ok := errors.AsType[*github.AbuseRateLimitError](err); ok {
		var retryAfter time.Duration
		if abuse.RetryAfter != nil {
			retryAfter = *abuse.RetryAfter
		}

		return &RateLimitError{
			RetryAfter: retryAfter,
			Err:        fmt.Errorf("%w: %w", httperr.ErrRateLimit, err),
		}
	}

	// Generic HTTP classification.
	if ghErr, ok := errors.AsType[*github.ErrorResponse](err); ok && ghErr.Response != nil {
		return httperr.ClassifyStatus(ghErr.Response.StatusCode, err)
	}

	return httperr.Classify(err)
}
