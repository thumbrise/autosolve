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

package otel_test

import (
	"context"
	"errors"
	"testing"
	"time"

	rsotel "github.com/thumbrise/autosolve/pkg/resilience/otel"
)

func TestPlugin_Name(t *testing.T) {
	p := rsotel.Plugin()

	if p.Name() != "otel" {
		t.Fatalf("expected name %q, got %q", "otel", p.Name())
	}
}

func TestPlugin_Events_OnAfterCallPopulated(t *testing.T) {
	p := rsotel.Plugin()
	events := p.Events()

	if events.OnAfterCall == nil {
		t.Fatal("expected OnAfterCall to be populated")
	}

	// Should not panic when called.
	events.OnAfterCall(context.Background(), 0, nil, 1*time.Second)
}

func TestPlugin_Events_OnAfterCallWithError(t *testing.T) {
	p := rsotel.Plugin()
	events := p.Events()

	// Should not panic when called with an error.
	events.OnAfterCall(context.Background(), 0, errors.New("fail"), 1*time.Second)
}

func TestPlugin_Events_OnBeforeWaitPopulated(t *testing.T) {
	p := rsotel.Plugin()
	events := p.Events()

	if events.OnBeforeWait == nil {
		t.Fatal("expected OnBeforeWait to be populated")
	}

	// Should not panic when called.
	events.OnBeforeWait(context.Background(), "node", 0, 2*time.Second)
}

func TestPlugin_Events_OnBeforeCallNil(t *testing.T) {
	p := rsotel.Plugin()
	events := p.Events()

	// OnBeforeCall is not used by the OTEL plugin — should be nil.
	if events.OnBeforeCall != nil {
		t.Fatal("expected OnBeforeCall to be nil")
	}
}
