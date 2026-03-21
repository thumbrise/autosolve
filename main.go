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
	"os"

	"github.com/thumbrise/autosolve/internal/bootstrap/wire"
	"github.com/thumbrise/autosolve/internal/infrastructure/config"
)

const envPrefix = "AUTOSOLVE"

func main() {
	ctx := context.Background()

	c, err := wire.InitializeContainer(ctx)
	if err != nil {
		log.Fatalf("cannot initialize container: %s", err.Error())
	}

	c.LoggerLoader.Load(ctx, true)

	err = c.ConfigLoader.Load(config.LoadOptions{
		EnvPrefix: envPrefix,
		File: &config.LoadOptionsFile{
			Path: ".",
			Name: "config",
			Type: "yml",
		},
	})
	if err != nil {
		log.Fatalf("cannot load config: %s", err)
	}

	err = c.Kernel.Execute(ctx)
	if err != nil {
		os.Exit(1)
	}
}
