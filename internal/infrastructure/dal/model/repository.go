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

type Repository struct {
	Record
	Owner     string  `gorm:"type:varchar(255);not null;uniqueIndex:idx_owner_name"`
	Name      string  `gorm:"type:varchar(255);not null;uniqueIndex:idx_owner_name"`
	Enabled   bool    `gorm:"default:true;index"`
	LastError *string `gorm:"type:text"`
}
