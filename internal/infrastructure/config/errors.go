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
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type MissingVariableError struct {
	variable string
}

func NewMissingVariable(variable string) *MissingVariableError {
	return &MissingVariableError{variable: variable}
}

func (m MissingVariableError) Error() string {
	return fmt.Sprintf("missing required environment variable %s. Please set it (e.g., export %s=xxx) and try again", m.variable, m.variable)
}

type InvalidVariableError struct {
	variable string
}

func NewInvalidVariableError(variable string) *InvalidVariableError {
	return &InvalidVariableError{variable: variable}
}

func (m InvalidVariableError) Error() string {
	return fmt.Sprintf("variable '%s' fail validation rule", m.variable)
}

func mapValidationErr(err error, viperKey string) error {
	var validationErrors validator.ValidationErrors

	ok := errors.As(err, &validationErrors)
	if !ok {
		return err
	}

	var result error
	for _, fe := range validationErrors {
		result = errors.Join(result, mapFieldErr(fe, viperKey))
	}

	return result
}

func mapFieldErr(fe validator.FieldError, viperKey string) error {
	if fe.Tag() == "" {
		return fe
	}

	_, varName, _ := strings.Cut(fe.StructNamespace(), ".")

	if viperKey != "" {
		varName = viperKey + "." + varName
	}

	varName = strings.ToUpper(strings.ReplaceAll(varName, ".", "_"))
	if prefix := viper.GetEnvPrefix(); prefix != "" {
		varName = prefix + "_" + varName
	}

	if fe.Tag() == "required" {
		return NewMissingVariable(varName)
	}

	return fmt.Errorf("%w: %w", NewInvalidVariableError(varName), fe)
}
