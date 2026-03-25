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
	"github.com/thumbrise/autosolve/internal/infrastructure/telemetry"
)

type Boot struct {
	ConfigReader *configinfra.Reader
	Logger       *slog.Logger
	ConfigLog    *config.Log
	ConfigOtel   *config.Otel
	Telemetry    *telemetry.Telemetry
}

var (
	ErrBootstrapConfig    = errors.New("cannot bootstrap config")
	ErrBootstrapTelemetry = errors.New("cannot bootstrap telemetry")
	ErrConfigLoad         = errors.New("cannot load config")
	ErrConfigAppRead      = errors.New("cannot read app config")
	ErrConfigOtelRead     = errors.New("cannot read otel config")
)

func Bootstrap(ctx context.Context) (*Boot, error) {
	b := &Boot{}

	reader, cfgLog, cfgOtel, err := b.bootstrapConfig(ctx, loggerinfra.New())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBootstrapConfig, err)
	}

	baseLogger, logger := b.bootstrapLogger(ctx, cfgOtel, cfgLog)

	reader.SetLogger(logger)

	tel, err := b.bootstrapTelemetry(ctx, cfgOtel, baseLogger)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBootstrapTelemetry, err)
	}

	b.ConfigReader = reader
	b.ConfigLog = cfgLog
	b.ConfigOtel = cfgOtel
	b.Logger = logger
	b.Telemetry = tel

	return b, nil
}

//nolint:nonamedreturns // 2 loggers
func (b *Boot) bootstrapLogger(ctx context.Context, cfgOtel *config.Otel, cfgLog *config.Log) (base *slog.Logger, full *slog.Logger) {
	base = loggerinfra.WithConfig(ctx, *cfgLog)

	full = base
	if cfgOtel.Logs.Exporter != "none" && cfgOtel.Logs.Exporter != "" && !cfgOtel.SDKDisabled {
		full = loggerinfra.WithOtelBridge(base, cfgOtel.ServiceName)
	}

	slog.SetDefault(full)

	return base, full
}

func (b *Boot) bootstrapConfig(ctx context.Context, logger *slog.Logger) (*configinfra.Reader, *config.Log, *config.Otel, error) {
	loader := configinfra.NewLoader(logger, configinfra.NewViper(logger))

	err := loader.Load(configinfra.LoadOptions{
		EnvPrefix: envPrefix,
		File: &configinfra.LoadOptionsFile{
			Path: ".",
			Name: "config",
			Type: "yml",
		},
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %w", ErrConfigLoad, err)
	}

	reader := loader.GetReader()

	cfgLog, err := config.NewLog(ctx, reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %w", ErrConfigAppRead, err)
	}

	cfgOtel, err := config.NewOtel(ctx, reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %w", ErrConfigOtelRead, err)
	}

	return reader, cfgLog, cfgOtel, nil
}

func (b *Boot) bootstrapTelemetry(ctx context.Context, cfg *config.Otel, logger *slog.Logger) (*telemetry.Telemetry, error) {
	return telemetry.New(ctx, cfg, logger)
}
