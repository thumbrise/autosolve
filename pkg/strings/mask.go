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

package strings

import (
	"strings"
	"unicode/utf8"
)

// MaskPercent masks the first `percent` percent of the `s` runes
// with the given `symbol`, leaving the rest visible.
func MaskPercent(s string, symbol rune, percent int) string {
	if percent <= 0 {
		return s
	}

	if percent >= 100 {
		return strings.Repeat(string(symbol), utf8.RuneCountInString(s))
	}

	runes := []rune(s)
	n := len(runes)

	maskCount := n * percent / 100
	if maskCount > n {
		maskCount = n
	}

	return strings.Repeat(string(symbol), maskCount) + string(runes[maskCount:])
}
