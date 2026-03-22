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

package github

import (
	"context"
	"net/http"

	"github.com/google/go-github/v84/github"

	"github.com/thumbrise/autosolve/internal/config"
)

func NewGithubClient(ctx context.Context, cfg *config.Github) *github.Client {
	httpClient := &http.Client{
		Transport: http.DefaultTransport,
		Timeout:   cfg.Issues.HttpClientTimeout,
	}

	return github.NewClient(httpClient).WithAuthToken(cfg.Token)
}
