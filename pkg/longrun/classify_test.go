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

package longrun_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"testing"

	"github.com/thumbrise/autosolve/pkg/longrun"
)

func TestClassifyTransport_URLError_Timeout(t *testing.T) {
	err := &url.Error{
		Op:  "Get",
		URL: "https://api.github.com",
		Err: &timeoutError{},
	}

	class := longrun.ClassifyTransport(err)
	if class == nil {
		t.Fatal("expected non-nil class")
	}

	if class.Category != longrun.CategoryNode {
		t.Fatalf("expected CategoryNode, got %d", class.Category)
	}
}

func TestClassifyTransport_URLError_NonTimeout(t *testing.T) {
	// url.Error wrapping net.OpError but Timeout() is false → falls through to net.OpError check.
	err := &url.Error{
		Op:  "Get",
		URL: "https://api.github.com",
		Err: &net.OpError{Op: "dial", Err: errors.New("connection refused")},
	}

	class := longrun.ClassifyTransport(err)
	if class == nil {
		t.Fatal("expected non-nil class for url.Error wrapping net.OpError")
	}

	if class.Category != longrun.CategoryNode {
		t.Fatalf("expected CategoryNode, got %d", class.Category)
	}
}

func TestClassifyTransport_ContextDeadlineExceeded(t *testing.T) {
	class := longrun.ClassifyTransport(context.DeadlineExceeded)
	if class == nil {
		t.Fatal("expected non-nil class")
	}

	if class.Category != longrun.CategoryNode {
		t.Fatalf("expected CategoryNode, got %d", class.Category)
	}
}

func TestClassifyTransport_NetOpError(t *testing.T) {
	err := &net.OpError{
		Op:  "dial",
		Err: errors.New("connection refused"),
	}

	class := longrun.ClassifyTransport(err)
	if class == nil {
		t.Fatal("expected non-nil class")
	}

	if class.Category != longrun.CategoryNode {
		t.Fatalf("expected CategoryNode, got %d", class.Category)
	}
}

func TestClassifyTransport_DNSError(t *testing.T) {
	err := &net.DNSError{
		Name: "api.github.com",
		Err:  "no such host",
	}

	class := longrun.ClassifyTransport(err)
	if class == nil {
		t.Fatal("expected non-nil class")
	}

	if class.Category != longrun.CategoryNode {
		t.Fatalf("expected CategoryNode, got %d", class.Category)
	}
}

func TestClassifyTransport_EOF(t *testing.T) {
	class := longrun.ClassifyTransport(io.EOF)
	if class == nil {
		t.Fatal("expected non-nil class")
	}

	if class.Category != longrun.CategoryNode {
		t.Fatalf("expected CategoryNode, got %d", class.Category)
	}
}

func TestClassifyTransport_UnexpectedEOF(t *testing.T) {
	class := longrun.ClassifyTransport(io.ErrUnexpectedEOF)
	if class == nil {
		t.Fatal("expected non-nil class")
	}

	if class.Category != longrun.CategoryNode {
		t.Fatalf("expected CategoryNode, got %d", class.Category)
	}
}

func TestClassifyTransport_WrappedNetOpError(t *testing.T) {
	inner := &net.OpError{Op: "dial", Err: errors.New("refused")}
	wrapped := fmt.Errorf("http client: %w", inner)

	class := longrun.ClassifyTransport(wrapped)
	if class == nil {
		t.Fatal("expected non-nil class for wrapped net.OpError")
	}

	if class.Category != longrun.CategoryNode {
		t.Fatalf("expected CategoryNode, got %d", class.Category)
	}
}

func TestClassifyTransport_UnknownError(t *testing.T) {
	err := errors.New("something unexpected")

	class := longrun.ClassifyTransport(err)
	if class != nil {
		t.Fatalf("expected nil for unknown error, got category %d", class.Category)
	}
}

func TestClassifyTransport_Nil(t *testing.T) {
	class := longrun.ClassifyTransport(nil)
	if class != nil {
		t.Fatal("expected nil for nil error")
	}
}

// timeoutError implements the Timeout() interface for url.Error.
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }
