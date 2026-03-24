package github

import (
	"errors"
	"fmt"

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
	// GitHub-специфика: rate limit приходит как 403, а не 429.
	var rateLimitErr *github.RateLimitError
	if errors.As(err, &rateLimitErr) {
		return fmt.Errorf("%w: %w", httperr.ErrRateLimit, err)
	}

	var abuseErr *github.AbuseRateLimitError
	if errors.As(err, &abuseErr) {
		return fmt.Errorf("%w: %w", httperr.ErrRateLimit, err)
	}
	// Generic HTTP classification.
	var ghErr *github.ErrorResponse
	if errors.As(err, &ghErr) && ghErr.Response != nil {
		return httperr.ClassifyStatus(ghErr.Response.StatusCode, err)
	}

	return httperr.Classify(err)
}
