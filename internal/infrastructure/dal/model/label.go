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

type Label struct {
	Record
	RepositoryID uint64 `gorm:"not null;index"`
	GithubID     int64  `gorm:"not null;uniqueIndex"`
	Name         string `gorm:"type:varchar(255);not null"`
	Color        string `gorm:"type:varchar(10)"`
	Description  string `gorm:"type:text"`
	// Relations
	Repository *Repository `gorm:"foreignKey:RepositoryID"`
}
