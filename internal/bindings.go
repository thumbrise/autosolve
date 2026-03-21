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

package internal

import (
	"github.com/google/wire"

	"github.com/thumbrise/autosolve/internal/application"
	"github.com/thumbrise/autosolve/internal/application/issue"
	"github.com/thumbrise/autosolve/internal/bootstrap"
	"github.com/thumbrise/autosolve/internal/bootstrap/contracts"
	"github.com/thumbrise/autosolve/internal/infrastructure/config"
	"github.com/thumbrise/autosolve/internal/infrastructure/logger"
)

var Bindings = wire.NewSet(
	bootstrap.NewKernel,
	wire.Bind(
		new(contracts.RootCMD),
		new(*bootstrap.Kernel),
	),

	logger.NewSlogLogger,
	logger.NewLoader,

	config.NewViper,
	config.NewValidator,
	config.NewLoader,
	config.NewReader,

	application.NewScheduler,
	issue.NewWorker,
)
