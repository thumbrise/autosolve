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

package application

import (
	"context"

	"github.com/thumbrise/autosolve/internal/application/issue"
	"github.com/thumbrise/autosolve/pkg/longrun"
)

type Scheduler struct{}

func NewScheduler() *Scheduler {
	return &Scheduler{}
}

func (s *Scheduler) Run(ctx context.Context) error {
	runner := longrun.NewRunner()

	runner.Add(&longrun.Process{
		Name: "polling issues",
		Start: func(ctx context.Context) error {
			return issue.NewWorker().Run(ctx)
		},
		Shutdown: nil,
	})

	return runner.Wait(ctx)
	// classifier := retrier.WhitelistClassifier{}
	// retrier.New(retrier.ExponentialBackoff(10, 2*time.Second))
	// bexp := backoff.NewExponentialBackOff()
	// bexp.MaxInterval = 30 * time.Second
	// bexp.MaxElapsedTime = 0
	// b := backoff.WithMaxRetries(bexp, 50)

	// return nil
}
