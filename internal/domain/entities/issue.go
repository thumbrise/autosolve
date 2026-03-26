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

package entities

import (
	"time"
)

const (
	IssueStateOpen   = "open"
	IssueStateClosed = "closed"
)

//nolint:godox // schema reference
// TODO(v1-epic): Record will be removed when all entities migrate to sqlc-generated models.

type Issue struct {
	Record
	RepositoryID    int64
	GithubID        int64
	Number          int64
	Title           string
	Body            string
	State           string
	IsPullRequest   bool
	PRUrl           *string
	PRHtmlUrl       *string
	PRDiffUrl       *string
	PRPatchUrl      *string
	GithubCreatedAt time.Time
	GithubUpdatedAt time.Time
	SyncedAt        time.Time
	// Relations
	Repository *Repository
	Labels     []*Label
	Assignees  []*User
	Comments   []*Comment
}
