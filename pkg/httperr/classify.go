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

package httperr

import (
	"errors"
	"fmt"
	"net"
	"net/url"
)

// StatusCoder is implemented by HTTP error types that carry a status code.
// Standard library does not define this interface, but many HTTP clients
// (including go-github) return errors with a Response.StatusCode field.
//
// Implement this interface on your HTTP error types to enable automatic
// classification by Classify.
type StatusCoder interface {
	StatusCode() int
}

// Classify wraps err with a transient sentinel when the error is objectively
// retryable per HTTP/REST semantics. Permanent errors are returned as-is.
//
// Classification order:
//  1. nil → nil
//  2. context.DeadlineExceeded or *url.Error with timeout → ErrTimeout
//  3. *net.OpError or *net.DNSError → ErrNetwork
//  4. StatusCoder with 408 → ErrTimeout
//  5. StatusCoder with 429 → ErrRateLimit
//  6. StatusCoder with 5xx → ErrServerError
//  7. Everything else → returned unchanged (permanent)
//
// The original error is always preserved in the chain via fmt.Errorf("%w: %w"),
// so callers can still use errors.As to access the underlying typed error.
//
// Example:
//
//	resp, err := httpClient.Do(req)
//	if err != nil {
//	    return httperr.Classify(err)
//	}
func Classify(err error) error {
	if err == nil {
		return nil
	}
	// Timeout: url.Error wraps both client-side deadlines and dial timeouts.
	// Check before net.OpError because url.Error often wraps net.OpError.
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Timeout() {
		return fmt.Errorf("%w: %w", ErrTimeout, err)
	}
	// Network: TCP, DNS, TLS failures.
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return fmt.Errorf("%w: %w", ErrNetwork, err)
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return fmt.Errorf("%w: %w", ErrNetwork, err)
	}
	// HTTP status codes via StatusCoder interface.
	var sc StatusCoder
	if errors.As(err, &sc) {
		return ClassifyStatus(sc.StatusCode(), err)
	}
	// Unknown — return as-is (permanent).
	return err
}

// ClassifyStatus maps an HTTP status code to a transient sentinel.
// Non-transient codes return the original error unchanged.
func ClassifyStatus(code int, err error) error {
	switch {
	case code == 408:
		return fmt.Errorf("%w: %w", ErrTimeout, err)
	case code == 429:
		return fmt.Errorf("%w: %w", ErrRateLimit, err)
	case code >= 500:
		return fmt.Errorf("%w: %w", ErrServerError, err)
	default:
		return err
	}
}

// TransientErrors returns all sentinel errors that represent objectively
// transient HTTP failures per REST semantics.
//
// Useful for building github.com/thumbrise/longrun or custom retry logic
// without depending on pkg/longrun directly.
//
// Returned errors: ErrNetwork, ErrTimeout, ErrRateLimit, ErrServerError.
func TransientErrors() []error {
	return []error{
		ErrNetwork,
		ErrTimeout,
		ErrRateLimit,
		ErrServerError,
	}
}
