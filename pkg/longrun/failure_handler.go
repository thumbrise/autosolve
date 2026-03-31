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
)

// errSkip signals that this handler does not match the error.
// The pipeline moves to the next handler.
var errSkip = errors.New("longrun: handler skip")

// failureHandler processes a work error and decides: retry or stop.
//
// Two implementations:
//   - ruleFailureHandler: per-error matching via errors.Is/As (from TransientRules)
//   - baselineFailureHandler: classification pipeline (from Baseline)
//
// Contract:
//   - Return errSkip → this handler doesn't match, try the next one.
//   - Return nil → error handled, caller should retry.
//   - Return non-nil (not errSkip) → permanent error, stop.
type failureHandler interface {
	// Handle processes the error. See contract above.
	Handle(ctx context.Context, err error) error
}
