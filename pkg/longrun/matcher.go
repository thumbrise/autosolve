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
	"errors"
	"fmt"
	"reflect"
)

// Matcher checks whether an error matches a given pattern.
//
// Two forms are supported:
//   - error value (sentinel): matched via errors.Is
//   - *T where T implements error: matched via errors.As
//
// Examples:
//
//	NewMatcher(ErrTimeout)          // sentinel → errors.Is
//	NewMatcher((*net.OpError)(nil)) // pointer-to-type → errors.As
type Matcher struct {
	match func(error) bool
}

// NewMatcher compiles an error pattern into a Matcher.
//
// The errVal argument must be one of:
//   - an error value (for errors.Is matching)
//   - a pointer to an error type, i.e. *T where T implements error (for errors.As matching)
//
// Panics if errVal is nil or an unsupported type.
func NewMatcher(errVal any) Matcher {
	if errVal == nil {
		panic("longrun.NewMatcher: errVal must not be nil")
	}

	// Case 1: *T where T implements error → errors.As (type matching).
	// Must be checked BEFORE the error interface check, because a typed nil
	// pointer like (*net.OpError)(nil) satisfies the error interface but
	// should be matched by type, not by identity.
	rv := reflect.ValueOf(errVal)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		errorIface := reflect.TypeOf((*error)(nil)).Elem()
		if rv.Type().Implements(errorIface) {
			targetType := rv.Type()

			// errors.As requires a **T target (pointer to pointer).
			// targetType is *T, so reflect.New(targetType) gives us **T.
			return Matcher{match: func(err error) bool {
				target := reflect.New(targetType)

				return errors.As(err, target.Interface())
			}}
		}
	}

	// Case 2: error value (sentinel) → errors.Is.
	if sentinel, ok := errVal.(error); ok {
		return Matcher{match: func(err error) bool {
			return errors.Is(err, sentinel)
		}}
	}

	panic(fmt.Sprintf("longrun.NewMatcher: errVal must be an error value or pointer to error type (*T), got: %T", errVal))
}

// Match reports whether err matches the pattern.
func (m Matcher) Match(err error) bool {
	return m.match(err)
}
