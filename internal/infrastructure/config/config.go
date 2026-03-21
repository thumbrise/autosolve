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
	"fmt"
	"log/slog"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

// Read method recognize config variables from registered file or environment and unmarshal to out.
//
// out must be pointer to a struct variable.
//
// If key is empty trying unmarshal whole config from root key as base.
// See config file registered via config.Load. See viper.Unmarshal, viper.UnmarshalKey,
//
// You can validate struct fields via go-playground/validator struct tags. For example `validate:required`. See validator.Validate.
//
// You can mask secret values for slog via struct tags `masq:secret`. See masq.New, logger.Configure
func Read(ctx context.Context, out interface{}, key string) error {
	var err error

	if key == "" {
		err = viper.Unmarshal(out)
	} else {
		err = viper.UnmarshalKey(key, out)
	}

	if err != nil {
		return fmt.Errorf("failed to unmarshal cfg: %w", err)
	}

	err = validate.Struct(out)
	if err != nil {
		return fmt.Errorf("failed to validate config: %w", mapValidationErr(err, key))
	}

	slog.DebugContext(ctx, "Loaded config", "config", out)

	return nil
}
