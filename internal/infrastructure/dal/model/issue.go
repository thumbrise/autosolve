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

	"gorm.io/gorm"
)

const (
	IssueStateOpen   = "open"
	IssueStateClosed = "closed"
)

const (
	IssueProcessingStatusPending    = uint8(1)
	IssueProcessingStatusProcessing = uint8(2)
	IssueProcessingStatusDone       = uint8(3)
	IssueProcessingStatusFailed     = uint8(4)
)

type IssueLabel struct {
	ID          int64
	URL         string
	Name        string
	Color       string
	Description string
	NodeID      string
	Default     bool
}

type IssueAssignee struct {
	ID    int64
	Login string
}

// Issue represents a GitHub issue stored locally.
type Issue struct {
	gorm.Model

	// GitHub ID (unique)
	IssueID int64 `gorm:"uniqueIndex;not null"`

	// Issue title
	Title string `gorm:"type:text"`

	// Issue body (description)
	Body string `gorm:"type:text"`

	// State: IssueStateOpen "open" or IssueStateClosed "closed"
	State string `gorm:"type:varchar(10);index"`

	// Processing priority (higher = more important)
	Priority int `gorm:"default:0;index"`

	// Processing status: IssueProcessingStatusPending, IssueProcessingStatusProcessing, IssueProcessingStatusDone, IssueProcessingStatusFailed
	Status uint8 `gorm:"default:1;index"`

	// Number of processing attempts
	Attempts int `gorm:"default:0"`

	// Last processing error message, if any
	LastProcessingError *string `gorm:"type:text"`

	// Last time when the issue was successfully processed (nil = not processed)
	ProcessedAt *time.Time `gorm:"index"`

	// Last time when issue was retrieved from GitHub with actualized information
	SyncedAt *time.Time `gorm:"index"`

	// GitHub creation time
	GithubCreatedAt time.Time `gorm:"index"`

	// When repository issue was updated.
	//
	// Note: To not breaks convention with default CreatedAt, UpdatedAt using GitHub prefix.
	GithubUpdatedAt time.Time `gorm:"index;not null"`

	// Labels
	Labels []*IssueLabel `gorm:"type:json;serializer:json"`

	// Assignees
	Assignees []*IssueAssignee `gorm:"type:json;serializer:json"`
}
