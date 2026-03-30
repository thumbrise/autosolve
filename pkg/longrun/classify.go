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

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
)

// ClassifyTransport checks whether err is a transport-level failure.
// Returns CategoryNode for network and timeout errors, nil otherwise.
//
// This is a built-in classifier that depends only on stdlib.
// It runs before any user-provided ClassifierFunc in the handleFailure pipeline.
//
// Exported for testability and for use in custom ClassifierFunc implementations
// that want to extend (not replace) the built-in transport classification.
//
// Classification rules:
//   - url.Error with Timeout() → Node (check before net.OpError because url.Error often wraps it)
//   - context.DeadlineExceeded → Node
//   - net.OpError → Node
//   - net.DNSError → Node
//   - io.EOF, io.ErrUnexpectedEOF → Node (connection dropped mid-response)
func ClassifyTransport(err error) *ErrorClass {
	// Timeout: url.Error wraps both client-side deadlines and dial timeouts.
	// Check before net.OpError because url.Error often wraps net.OpError.
	if urlErr, ok := errors.AsType[*url.Error](err); ok && urlErr.Timeout() {
		return &ErrorClass{Category: CategoryNode}
	}

	// context.DeadlineExceeded — request exceeded its deadline.
	if errors.Is(err, context.DeadlineExceeded) {
		return &ErrorClass{Category: CategoryNode}
	}

	// Network: TCP, DNS, TLS failures.
	if _, ok := errors.AsType[*net.OpError](err); ok {
		return &ErrorClass{Category: CategoryNode}
	}

	if _, ok := errors.AsType[*net.DNSError](err); ok {
		return &ErrorClass{Category: CategoryNode}
	}

	// EOF: connection dropped mid-response.
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return &ErrorClass{Category: CategoryNode}
	}

	return nil
}
