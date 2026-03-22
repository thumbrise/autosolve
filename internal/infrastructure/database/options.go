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

package database

import (
	"fmt"
	"slices"
	"strings"
)

type SQLiteOptions struct {
	Path   string
	Params map[string]string
	Pragma map[string]string
}

func (o SQLiteOptions) DSN() string {
	parts := make([]string, 0, len(o.Params)+len(o.Pragma))
	for _, k := range sorted(o.Params) {
		parts = append(parts, fmt.Sprintf("%s=%s", k, o.Params[k]))
	}

	for _, k := range sorted(o.Pragma) {
		parts = append(parts, fmt.Sprintf("_pragma=%s(%s)", k, o.Pragma[k]))
	}

	if len(parts) == 0 {
		return o.Path
	}

	return o.Path + "?" + strings.Join(parts, "&")
}

func sorted(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	return keys
}
