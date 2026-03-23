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

package main

import (
	"context"
	"log"

	"github.com/thumbrise/autosolve/internal/bootstrap"
)

func main() {
	ctx := context.Background()

	boot, err := bootstrap.Bootstrap(ctx)
	if err != nil {
		log.Fatalf("failed to bootstrap: %s", err)
	}

	kernel, err := bootstrap.InitializeKernel(
		ctx,
		boot.ConfigReader,
		boot.ConfigLog,
		boot.Logger,
		boot.Telemetry,
	)
	if err != nil {
		log.Fatalf("failed initialize kernel: %s", err)
	}

	err = kernel.Execute(ctx)
	if err != nil {
		log.Fatalf("execution failed: %s", err)
	}
}
