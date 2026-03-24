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

package httperr_test

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"

	"github.com/thumbrise/autosolve/pkg/httperr"
)

// --- helpers ---
type statusError struct {
	code int
	msg  string
}

func (e *statusError) Error() string   { return e.msg }
func (e *statusError) StatusCode() int { return e.code }

// --- nil ---
func TestClassify_Nil(t *testing.T) {
	if err := httperr.Classify(nil); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

// --- timeout ---
func TestClassify_URLError_Timeout(t *testing.T) {
	original := &url.Error{
		Op:  "Get",
		URL: "https://api.github.com",
		Err: &timeoutError{},
	}

	err := httperr.Classify(original)
	if !errors.Is(err, httperr.ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got: %v", err)
	}

	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		t.Fatal("original *url.Error should be preserved in chain")
	}
}

func TestClassify_URLError_NonTimeout(t *testing.T) {
	original := &url.Error{
		Op:  "Get",
		URL: "https://api.github.com",
		Err: &net.OpError{Op: "dial", Err: errors.New("connection refused")},
	}
	err := httperr.Classify(original)
	// url.Error wraps net.OpError, but Timeout() is false → falls through to net.OpError check.
	if !errors.Is(err, httperr.ErrNetwork) {
		t.Fatalf("expected ErrNetwork for non-timeout url.Error wrapping net.OpError, got: %v", err)
	}
}

// --- network ---
func TestClassify_NetOpError(t *testing.T) {
	original := &net.OpError{
		Op:  "dial",
		Err: errors.New("connection refused"),
	}

	err := httperr.Classify(original)
	if !errors.Is(err, httperr.ErrNetwork) {
		t.Fatalf("expected ErrNetwork, got: %v", err)
	}

	var netErr *net.OpError
	if !errors.As(err, &netErr) {
		t.Fatal("original *net.OpError should be preserved in chain")
	}
}

func TestClassify_DNSError(t *testing.T) {
	original := &net.DNSError{
		Name: "api.github.com",
		Err:  "no such host",
	}

	err := httperr.Classify(original)
	if !errors.Is(err, httperr.ErrNetwork) {
		t.Fatalf("expected ErrNetwork, got: %v", err)
	}

	var dnsErr *net.DNSError
	if !errors.As(err, &dnsErr) {
		t.Fatal("original *net.DNSError should be preserved in chain")
	}
}

// --- status codes ---
func TestClassify_Status408(t *testing.T) {
	original := &statusError{code: 408, msg: "request timeout"}

	err := httperr.Classify(original)
	if !errors.Is(err, httperr.ErrTimeout) {
		t.Fatalf("expected ErrTimeout for 408, got: %v", err)
	}
}

func TestClassify_Status429(t *testing.T) {
	original := &statusError{code: 429, msg: "too many requests"}

	err := httperr.Classify(original)
	if !errors.Is(err, httperr.ErrRateLimit) {
		t.Fatalf("expected ErrRateLimit for 429, got: %v", err)
	}
}

func TestClassify_Status500(t *testing.T) {
	original := &statusError{code: 500, msg: "internal server error"}

	err := httperr.Classify(original)
	if !errors.Is(err, httperr.ErrServerError) {
		t.Fatalf("expected ErrServerError for 500, got: %v", err)
	}
}

func TestClassify_Status502(t *testing.T) {
	original := &statusError{code: 502, msg: "bad gateway"}

	err := httperr.Classify(original)
	if !errors.Is(err, httperr.ErrServerError) {
		t.Fatalf("expected ErrServerError for 502, got: %v", err)
	}
}

func TestClassify_Status503(t *testing.T) {
	original := &statusError{code: 503, msg: "service unavailable"}

	err := httperr.Classify(original)
	if !errors.Is(err, httperr.ErrServerError) {
		t.Fatalf("expected ErrServerError for 503, got: %v", err)
	}
}

func TestClassify_Status504(t *testing.T) {
	original := &statusError{code: 504, msg: "gateway timeout"}

	err := httperr.Classify(original)
	if !errors.Is(err, httperr.ErrServerError) {
		t.Fatalf("expected ErrServerError for 504, got: %v", err)
	}
}

// --- permanent (unchanged) ---
func TestClassify_Status400_Permanent(t *testing.T) {
	original := &statusError{code: 400, msg: "bad request"}

	err := httperr.Classify(original)
	if errors.Is(err, httperr.ErrTimeout) || errors.Is(err, httperr.ErrRateLimit) || errors.Is(err, httperr.ErrServerError) || errors.Is(err, httperr.ErrNetwork) {
		t.Fatalf("400 should be permanent, got transient sentinel: %v", err)
	}

	if !errors.Is(err, original) {
		t.Fatalf("permanent error should be returned as-is, got: %v", err)
	}
}

func TestClassify_Status401_Permanent(t *testing.T) {
	original := &statusError{code: 401, msg: "unauthorized"}

	err := httperr.Classify(original)
	if !errors.Is(err, original) {
		t.Fatalf("401 should be returned as-is, got: %v", err)
	}
}

func TestClassify_Status404_Permanent(t *testing.T) {
	original := &statusError{code: 404, msg: "not found"}

	err := httperr.Classify(original)
	if !errors.Is(err, original) {
		t.Fatalf("404 should be returned as-is, got: %v", err)
	}
}

// --- unknown error ---
func TestClassify_UnknownError_Permanent(t *testing.T) {
	original := errors.New("something unexpected")

	err := httperr.Classify(original)
	if !errors.Is(err, original) {
		t.Fatalf("unknown error should be returned as-is, got: %v", err)
	}
}

// --- wrapped errors ---
func TestClassify_WrappedNetOpError(t *testing.T) {
	inner := &net.OpError{Op: "dial", Err: errors.New("refused")}
	wrapped := fmt.Errorf("http client: %w", inner)

	err := httperr.Classify(wrapped)
	if !errors.Is(err, httperr.ErrNetwork) {
		t.Fatalf("expected ErrNetwork for wrapped net.OpError, got: %v", err)
	}
}

func TestClassify_WrappedStatusCoder(t *testing.T) {
	inner := &statusError{code: 503, msg: "unavailable"}
	wrapped := fmt.Errorf("github api: %w", inner)

	err := httperr.Classify(wrapped)
	if !errors.Is(err, httperr.ErrServerError) {
		t.Fatalf("expected ErrServerError for wrapped 503, got: %v", err)
	}
}

// --- chain preservation ---
func TestClassify_PreservesOriginalInChain(t *testing.T) {
	original := &statusError{code: 429, msg: "rate limited"}
	err := httperr.Classify(original)

	var sc httperr.StatusCoder
	if !errors.As(err, &sc) {
		t.Fatal("original StatusCoder should be preserved in chain")
	}

	if sc.StatusCode() != 429 {
		t.Fatalf("expected status 429, got %d", sc.StatusCode())
	}
}

// --- timeoutError helper ---
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }
