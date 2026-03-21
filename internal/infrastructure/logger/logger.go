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

package logger

import (
	"log/slog"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/m-mizutani/masq"
)

// maskPercentHead masks the first `percent` percent of the string's runes
// with the given symbol, leaving the rest visible. It always returns true
// for string inputs and false otherwise.
func maskPercentHead(symbol rune, percent int) masq.Redactor {
	return masq.RedactString(func(s string) string {
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
	})
}

func Configure() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: masq.New(masq.WithTag("secret", maskPercentHead('*', 75))),
	})))
}
