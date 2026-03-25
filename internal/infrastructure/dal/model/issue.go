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

package model

import (
	"time"
)

const (
	IssueStateOpen   = "open"
	IssueStateClosed = "closed"
)

type Issue struct {
	Record
	// RepositoryID    uint64    `gorm:"not null;index"`
	RepositoryID    uint64    `gorm:"index"`
	GithubID        int64     `gorm:"not null;uniqueIndex"`
	Number          int       `gorm:"not null;index"`
	Title           string    `gorm:"type:text"`
	Body            string    `gorm:"type:text"`
	State           string    `gorm:"type:varchar(10);not null;index"` // open/closed
	IsPullRequest   bool      `gorm:"default:false;index"`
	PRUrl           *string   `gorm:"type:text"`
	PRHtmlUrl       *string   `gorm:"type:text"`
	PRDiffUrl       *string   `gorm:"type:text"`
	PRPatchUrl      *string   `gorm:"type:text"`
	GithubCreatedAt time.Time `gorm:"not null"`
	GithubUpdatedAt time.Time `gorm:"not null;index"`
	SyncedAt        time.Time `gorm:"not null;index"`
	// Relations
	// Repository *Repository `gorm:"foreignKey:RepositoryID"`
	Labels    []*Label   `gorm:"many2many:issue_labels;"`
	Assignees []*User    `gorm:"many2many:issue_assignees;"`
	Comments  []*Comment `gorm:"foreignKey:IssueID"`
}
