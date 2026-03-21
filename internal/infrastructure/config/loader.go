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

package config

import (
	"log/slog"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type Loader struct {
	viper     *viper.Viper
	validator *validator.Validate
	logger    *slog.Logger
}

func NewLoader(logger *slog.Logger, vp *viper.Viper) *Loader {
	return &Loader{
		viper:     vp,
		validator: validator.New(validator.WithRequiredStructEnabled()),
		logger:    logger,
	}
}

func (l *Loader) Load(opts LoadOptions) error {
	l.viper.SetEnvPrefix(opts.EnvPrefix)

	if opts.File != nil {
		l.viper.AddConfigPath(opts.File.Path)
		l.viper.SetConfigName(opts.File.Name)
		l.viper.SetConfigType(opts.File.Type)

		return l.viper.ReadInConfig()
	}

	return nil
}

func (l *Loader) GetReader() *Reader {
	return NewReader(l.logger, NewValidator(), l.viper)
}
