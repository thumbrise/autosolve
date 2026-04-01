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

package schedule

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"time"

	"github.com/thumbrise/autosolve/internal/contracts/apierr"
)

// isNodeError reports whether err is a transport-level failure
// (TCP, DNS, TLS, timeout, connection dropped).
func isNodeError(err error) bool {
	if urlErr, ok := errors.AsType[*url.Error](err); ok && urlErr.Timeout() {
		return true
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	if _, ok := errors.AsType[*net.OpError](err); ok {
		return true
	}

	if _, ok := errors.AsType[*net.DNSError](err); ok {
		return true
	}

	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

// isServiceError reports whether err indicates remote service pressure
// (rate limit, 5xx, maintenance).
func isServiceError(err error) bool {
	var wh apierr.WaitHinted
	if errors.As(err, &wh) && wh.WaitDuration() > 0 {
		return true
	}

	var sp apierr.ServicePressure
	if errors.As(err, &sp) && sp.ServicePressure() {
		return true
	}

	var rt apierr.Retryable
	if errors.As(err, &rt) && rt.Retryable() {
		return true
	}

	return false
}

// always matches any error. Used as catch-all for unregistered errors.
func always(error) bool { return true }

// serviceWaitHint extracts server-suggested wait duration from the error.
// Returns 0 when no hint is available — backoff is used as fallback.
func serviceWaitHint(err error) time.Duration {
	var wh apierr.WaitHinted
	if errors.As(err, &wh) {
		return wh.WaitDuration()
	}

	return 0
}
