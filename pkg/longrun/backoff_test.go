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
	"testing"
	"time"

	"github.com/thumbrise/autosolve/pkg/longrun"
)

func TestDefaultBackoff(t *testing.T) {
	b := longrun.DefaultBackoff()

	if b.Initial != 1*time.Second {
		t.Errorf("Initial = %v, want 1s", b.Initial)
	}

	if b.Max != 30*time.Second {
		t.Errorf("Max = %v, want 30s", b.Max)
	}

	if b.Multiplier != 2.0 {
		t.Errorf("Multiplier = %v, want 2.0", b.Multiplier)
	}

	if b.MaxRetries != 0 {
		t.Errorf("MaxRetries = %v, want 0", b.MaxRetries)
	}
}
