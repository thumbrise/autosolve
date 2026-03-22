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
	"log"
	"log/slog"

	"github.com/thumbrise/autosolve/internal/infrastructure/config"
)

func GetLoader() *config.Loader {
	logger := slog.Default()
	vp := config.NewViper(logger)
	loader := config.NewLoader(logger, vp)

	err := loader.Load(config.LoadOptions{
		File: &config.LoadOptionsFile{
			Path: ".",
			Name: "example",
			Type: "yml",
		},
	})
	if err != nil {
		fmt.Println(err)
	}

	return loader
}

func ExampleReader_Read() {
	type Params struct {
		ParamStr string `validate:"required"`
		ParamInt int
	}

	type Config struct {
		// Replaced with mask when use slog
		MyToken  string `masq:"secret" validate:"required"`
		MyParams Params
	}

	var cfg Config

	err := GetLoader().GetReader().Read(context.Background(), &cfg, "")
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

func ExampleRead() {
	type MyParams struct {
		ParamStr string `validate:"required"`
		ParamInt int
	}

	ctx := context.Background()
	reader := GetLoader().GetReader()

	cfg, err := config.Read[MyParams](ctx, reader)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(cfg.ParamStr)
	fmt.Println(cfg.ParamInt)
	// output:
	// param
	// 5
}

func ExampleRead_rootPrimitive() {
	type SomeRootStringParam string

	ctx := context.Background()
	reader := GetLoader().GetReader()

	vp, err := config.Read[SomeRootStringParam](ctx, reader)
	if err != nil {
		log.Fatal(err)
	}

	v := *vp
	fmt.Println(v)
	// output:
	// i'm root param
}
