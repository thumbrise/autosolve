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

package github

import "time"

// Cursor carries pagination and conditional-request state for a single API call.
// Built by the domain layer from SyncCursor; interpreted by Client.
type Cursor struct {
	ETag  string
	Limit int
	Page  int
	Since time.Time
}

// Request identifies the repository and cursor for a GitHub API call.
type Request struct {
	Owner      string
	Repository string
	Cursor     Cursor
}
