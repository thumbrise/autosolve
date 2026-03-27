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

import "github.com/thumbrise/autosolve/internal/domain/entities"

// Response is the domain-facing result of a GitHub API call.
// It isolates the domain from go-github types — *github.Response never leaves this package.
type Response struct {
	// Issues contains the fetched domain entities. Nil when NotModified is true.
	Issues []*entities.Issue

	// NextCursor is the recommended cursor for the next request.
	// The domain layer persists it as SyncCursor.
	NextCursor Cursor

	// NotModified is true when the server returned 304 (ETag matched).
	// Issues is nil and NextCursor is unchanged in this case.
	NotModified bool
}
