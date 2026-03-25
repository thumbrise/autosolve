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

import "time"

type Comment struct {
	Record
	IssueID         uint64    `gorm:"not null;index"`
	GithubID        int64     `gorm:"not null;uniqueIndex"`
	Body            string    `gorm:"type:text"`
	AuthorLogin     string    `gorm:"type:varchar(255)"`
	AuthorGithubID  int64     `gorm:"index"`
	GithubCreatedAt time.Time `gorm:"not null"`
	GithubUpdatedAt time.Time `gorm:"not null;index"`
	// Relations
	Issue *Issue `gorm:"foreignKey:IssueID"`
}
