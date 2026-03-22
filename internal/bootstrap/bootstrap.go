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

package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thumbrise/autosolve/internal/bootstrap/kernel"
	"github.com/thumbrise/autosolve/internal/config"
	"github.com/thumbrise/autosolve/internal/infrastructure/database"
)

const envPrefix = "AUTOSOLVE"

var (
	ErrConfigLoad      = errors.New("cannot load config")
	ErrDatabaseMigrate = errors.New("cannot migrate database")
)

type Bootstrapper struct {
	// Commands is consumed by Wire to trigger instantiation of all command providers.
	// Commands register themselves on the Kernel via side-effect in their constructors.
	Commands  []*cobra.Command
	Kernel    *kernel.Kernel
	Migrator  *database.Migrator
	ConfigApp *config.App
}

func NewBootstrapper(commands []*cobra.Command, configApp *config.App, kernel *kernel.Kernel, migrator *database.Migrator) *Bootstrapper {
	return &Bootstrapper{Commands: commands, ConfigApp: configApp, Kernel: kernel, Migrator: migrator}
}

func (b *Bootstrapper) InitializeKernel(ctx context.Context) (*kernel.Kernel, error) {
	err := b.Migrator.Migrate(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseMigrate, err)
	}

	return b.Kernel, nil
}
