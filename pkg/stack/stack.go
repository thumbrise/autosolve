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

package stack

import (
	"fmt"
	"runtime"
	"strings"
)

func Capture(skip int) []string {
	var pcs [32]uintptr

	n := runtime.Callers(skip, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	var stack []string

	for {
		frame, more := frames.Next()
		if strings.HasPrefix(frame.Function, "runtime.") ||
			strings.HasPrefix(frame.Function, "log/slog.") {
			if !more {
				break
			}

			continue
		}

		stack = append(stack, fmt.Sprintf("%s:%d", frame.File, frame.Line))

		if !more {
			break
		}
	}

	return stack
}
