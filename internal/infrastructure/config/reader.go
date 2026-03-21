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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type Reader struct {
	validator *Validator
	viper     *viper.Viper
	logger    *slog.Logger
}

func NewReader(logger *slog.Logger, validator *Validator, viper *viper.Viper) *Reader {
	return &Reader{logger: logger, validator: validator, viper: viper}
}

// Read method recognize config variables from registered file or environment and unmarshal to out.
//
// out must be pointer to a struct variable.
//
// If key is empty trying unmarshal whole config from root key as base.
// See config file registered via config.Load. See viper.Unmarshal, viper.UnmarshalKey,
//
// You can validate struct fields via go-playground/validator struct tags. For example `validate:required`. See validator.Validate.
//
// You can mask secret values for slog via struct tags `masq:secret`. See masq.New, logger.Load
func (c *Reader) Read(ctx context.Context, out interface{}, key string) error {
	var err error

	if key == "" {
		err = c.viper.Unmarshal(out)
	} else {
		err = c.viper.UnmarshalKey(key, out)
	}

	if err != nil {
		return fmt.Errorf("failed to unmarshal cfg: %w", err)
	}

	err = c.validator.Struct(out)
	if err != nil {
		return fmt.Errorf("failed to validate config: %w", c.mapValidationErr(err, key))
	}

	c.logger.DebugContext(ctx, "Loaded config", "config", out)

	return nil
}

func (c *Reader) mapValidationErr(err error, viperKey string) error {
	var validationErrors validator.ValidationErrors

	ok := errors.As(err, &validationErrors)
	if !ok {
		return err
	}

	var result error
	for _, fe := range validationErrors {
		result = errors.Join(result, c.mapFieldErr(fe, viperKey))
	}

	return result
}

func (c *Reader) mapFieldErr(fe validator.FieldError, viperKey string) error {
	if fe.Tag() == "" {
		return fe
	}

	_, varName, _ := strings.Cut(fe.StructNamespace(), ".")

	if viperKey != "" {
		varName = viperKey + "." + varName
	}

	varName = strings.ToUpper(strings.ReplaceAll(varName, ".", "_"))
	if prefix := c.viper.GetEnvPrefix(); prefix != "" {
		varName = prefix + "_" + varName
	}

	if fe.Tag() == "required" {
		return NewMissingVariable(varName)
	}

	return fmt.Errorf("%w: %w", NewInvalidVariableError(varName), fe)
}
