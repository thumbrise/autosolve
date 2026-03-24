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
	"errors"
	"fmt"
	"testing"

	"github.com/thumbrise/autosolve/pkg/longrun"
)

// --- test helpers ---

var errSentinel = errors.New("sentinel")

type typedError struct {
	Code int
}

func (e *typedError) Error() string {
	return fmt.Sprintf("typed error: %d", e.Code)
}

// --- sentinel matching ---

func TestMatcher_Sentinel_ExactMatch(t *testing.T) {
	m := longrun.NewMatcher(errSentinel)

	if !m.Match(errSentinel) {
		t.Fatal("expected match for exact sentinel")
	}
}

func TestMatcher_Sentinel_WrappedMatch(t *testing.T) {
	m := longrun.NewMatcher(errSentinel)

	if !m.Match(fmt.Errorf("wrapped: %w", errSentinel)) {
		t.Fatal("expected match for wrapped sentinel")
	}
}

func TestMatcher_Sentinel_NoMatch(t *testing.T) {
	m := longrun.NewMatcher(errSentinel)

	if m.Match(errors.New("other")) {
		t.Fatal("expected no match for different error")
	}
}

// --- pointer-to-type matching ---

func TestMatcher_PointerToType_DirectMatch(t *testing.T) {
	m := longrun.NewMatcher((*typedError)(nil))

	if !m.Match(&typedError{Code: 42}) {
		t.Fatal("expected match for *typedError")
	}
}

func TestMatcher_PointerToType_WrappedMatch(t *testing.T) {
	m := longrun.NewMatcher((*typedError)(nil))

	if !m.Match(fmt.Errorf("wrapped: %w", &typedError{Code: 1})) {
		t.Fatal("expected match for wrapped *typedError")
	}
}

func TestMatcher_PointerToType_NoMatch(t *testing.T) {
	m := longrun.NewMatcher((*typedError)(nil))

	if m.Match(errors.New("plain error")) {
		t.Fatal("expected no match for plain error")
	}
}

// --- panics ---

func TestNewMatcher_PanicsOnNil(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for nil")
		}
	}()

	longrun.NewMatcher(nil)
}

func TestNewMatcher_PanicsOnString(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for string")
		}
	}()

	longrun.NewMatcher("not an error")
}

func TestNewMatcher_PanicsOnInt(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for int")
		}
	}()

	longrun.NewMatcher(42)
}
