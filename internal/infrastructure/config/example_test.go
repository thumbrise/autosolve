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

package config_test

import (
	"context"
	"fmt"

	"github.com/thumbrise/autosolve/internal/infrastructure/config"
)

func ExampleRead() {
	type Params struct {
		ParamStr string `validate:"required"`
		ParamInt int
	}

	type Config struct {
		// Replaced with mask when use slog
		MyToken  string `masq:"secret" validate:"required"`
		MyParams Params
	}

	err := config.Load(config.LoadOptions{
		ConfigFilePath: ".",
		ConfigFileName: "example",
		ConfigFileType: "yml",
	})
	if err != nil {
		fmt.Println(err)
	}

	var cfg Config

	err = config.Read(context.Background(), &cfg, "")
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(cfg.MyToken)
	fmt.Println(cfg.MyParams.ParamStr)
	fmt.Println(cfg.MyParams.ParamInt)
	// output:
	// 1234-abcd-qwer-1w2w
	// param
	// 5
}
