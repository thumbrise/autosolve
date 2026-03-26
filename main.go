// Copyright 2026 thumbrise
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/thumbrise/autosolve/internal/bootstrap"
	"github.com/thumbrise/autosolve/internal/infrastructure/telemetry"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	code := run(ctx)

	cancel()
	os.Exit(code)
}

func run(ctx context.Context) int {
	boot, err := bootstrap.Bootstrap(ctx)
	if err != nil {
		log.Printf("failed to bootstrap: %s", err)

		return 1
	}
	defer func(Telemetry *telemetry.Telemetry, ctx context.Context) {
		_ = Telemetry.Shutdown(ctx)
	}(boot.Telemetry, ctx)

	kernel, err := bootstrap.InitializeKernel(
		ctx,
		boot.ConfigReader,
		boot.ConfigLog,
		boot.Logger,
	)
	if err != nil {
		return handleError(ctx, boot, fmt.Errorf("failed initialize kernel: %w", err))
	}

	var output bytes.Buffer

	err = kernel.Execute(ctx, &output)
	flushOutput(&output)

	if err != nil {
		return handleError(ctx, boot, fmt.Errorf("execution failed: %w", err))
	}

	return 0
}

func handleError(ctx context.Context, boot *bootstrap.Boot, err error) int {
	boot.Logger.ErrorContext(ctx, "fatal error", slog.Any("error", err))
	log.Println(err)

	return 1
}

func flushOutput(output *bytes.Buffer) {
	if output.Len() > 0 {
		fmt.Print(output.String())
	}
}
