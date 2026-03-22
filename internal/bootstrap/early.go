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
	"log/slog"

	"github.com/thumbrise/autosolve/internal/config"
	configinfra "github.com/thumbrise/autosolve/internal/infrastructure/config"
	loggerinfra "github.com/thumbrise/autosolve/internal/infrastructure/logger"
)

type EarlyBootstrapper struct {
	ConfigReader *configinfra.Reader
	Logger       *slog.Logger
	ConfigApp    *config.App
}

func NewEarlyBootstrapper() *EarlyBootstrapper {
	return &EarlyBootstrapper{}
}

var ErrConfigAppRead = errors.New("error reading app config")

func (b *EarlyBootstrapper) Bootstrap(ctx context.Context) error {
	b.Logger = loggerinfra.NewSlogLogger()

	err := b.bootstrapConfig(ctx)
	if err != nil {
		return err
	}

	b.bootstrapLogger(ctx)

	return nil
}

func (b *EarlyBootstrapper) bootstrapLogger(ctx context.Context) {
	loggerinfra.NewLoader().Load(ctx, b.ConfigApp.Debug)
}

func (b *EarlyBootstrapper) bootstrapConfig(ctx context.Context) error {
	loader := configinfra.NewLoader(b.Logger, configinfra.NewViper(b.Logger))

	err := loader.Load(configinfra.LoadOptions{
		EnvPrefix: envPrefix,
		File: &configinfra.LoadOptionsFile{
			Path: ".",
			Name: "config",
			Type: "yml",
		},
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrConfigLoad, err)
	}

	b.ConfigReader = loader.GetReader()

	b.ConfigApp, err = configinfra.Read[config.App](ctx, b.ConfigReader)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrConfigAppRead, err)
	}

	return nil
}
