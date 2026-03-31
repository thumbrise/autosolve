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

// AttemptStore tracks retry attempt counters.
//
// Default implementation is in-memory (MemoryStore). Users can provide

// a persistent implementation (Redis, SQLite) via WithAttemptStore to
// survive process restarts without losing backoff state.
//
// Keys are opaque strings formed by the caller — e.g. "rule:fetch issues"
// or "baseline:node". The store does not interpret them.
//
// Rule keys are derived from the error message (sentinels) or from the
// explicit TransientRule.Key field (typed nil pointers). This makes keys
// stable across deployments — reordering rules does not break persistent state.
//
// AttemptStore is NOT required to be safe for concurrent use.
// Each Task owns its own store instance.
type AttemptStore interface {
	// Increment advances the counter for key and returns the value
	// BEFORE the increment (0-based attempt index).
	Increment(key string) int

	// Get returns the current counter for key. Returns 0 for unknown keys.
	Get(key string) int

	// Reset sets all counters to zero.
	Reset()
}

// MemoryStore is the default in-memory AttemptStore.
// Exported so users can wrap it with decorators (logging, metrics, persistence fallback).
type MemoryStore struct {
	counters map[string]int
}

// NewMemoryStore creates an in-memory AttemptStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{counters: make(map[string]int)}
}

func (m *MemoryStore) Increment(key string) int {
	v := m.counters[key]
	m.counters[key] = v + 1

	return v
}

func (m *MemoryStore) Get(key string) int {
	return m.counters[key]
}

func (m *MemoryStore) Reset() {
	m.counters = make(map[string]int)
}
