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
	"github.com/spf13/cobra"

	"github.com/thumbrise/autosolve/internal/infrastructure/config"
	"github.com/thumbrise/autosolve/internal/infrastructure/logger"
)

type Container struct {
	// Commands is consumed by Wire to trigger instantiation of all command providers.
	// Commands register themselves on the Kernel via side-effect in their constructors.
	Commands     []*cobra.Command
	ConfigLoader *config.Loader
	LoggerLoader *logger.Loader
	Kernel       *Kernel
}

func NewContainer(commands []*cobra.Command, configLoader *config.Loader, kernel *Kernel, loggerLoader *logger.Loader) *Container {
	return &Container{Commands: commands, ConfigLoader: configLoader, Kernel: kernel, LoggerLoader: loggerLoader}
}
