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
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/thumbrise/autosolve/internal/bootstrap"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	exitCode := 0

	if err := run(ctx); err != nil {
		log.Println(err)

		exitCode = 1
	}

	cancel()
	os.Exit(exitCode)
}

func run(ctx context.Context) error {
	boot, err := bootstrap.Bootstrap(ctx)
	if err != nil {
		return fmt.Errorf("failed to bootstrap: %w", err)
	}

	kernel, err := bootstrap.InitializeKernel(
		ctx,
		boot.ConfigReader,
		boot.ConfigLog,
		boot.Logger,
		boot.Telemetry,
	)
	if err != nil {
		return fmt.Errorf("failed initialize kernel: %w", err)
	}

	err = kernel.Execute(ctx)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	return nil
}
