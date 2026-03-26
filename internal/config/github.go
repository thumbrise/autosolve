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
	"time"

	"github.com/thumbrise/autosolve/internal/infrastructure/config"
)

type Repository struct {
	Owner string `validate:"required"`
	Name  string `validate:"required"`
}

type Github struct {
	Token             string        `masq:"secret"            validate:"required"`
	Repositories      []Repository  `validate:"required,dive"`
	HttpClientTimeout time.Duration `validate:"required"`
	RateLimit         struct {
		MinInterval time.Duration `validate:"required"`
	} `validate:"required"`
	Issues struct {
		ParseInterval time.Duration `validate:"required"`
	}
}

func NewGithub(ctx context.Context, reader *config.Reader) (*Github, error) {
	return config.Read[Github](ctx, reader)
}
