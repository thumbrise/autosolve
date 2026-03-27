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

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/thumbrise/autosolve/internal/infrastructure/limit"
)

// Transport is an http.RoundTripper that ...
// TODO: add full responsibility description
//
//nolint:godox
type Transport struct {
	base      http.RoundTripper
	throttler *limit.MinIntervalThrottler
}

func NewTransport(throttler *limit.MinIntervalThrottler) *Transport {
	return &Transport{
		base:      otelhttp.NewTransport(http.DefaultTransport),
		throttler: throttler,
	}
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = setETagHeader(req)

	if err := t.throttler.Wait(req.Context()); err != nil {
		return nil, err
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if rateLimitConsumed(resp) {
		metricRateLimitConsumed.Add(req.Context(), 1)
	}

	return resp, nil
}

type etagKey struct{}

func setETagHeader(req *http.Request) *http.Request {
	if etag, ok := req.Context().Value(etagKey{}).(string); ok && etag != "" {
		req = req.Clone(req.Context())
		req.Header.Set("If-None-Match", etag)
	}

	return req
}

func withETagContext(ctx context.Context, etag string) context.Context {
	return context.WithValue(ctx, etagKey{}, etag)
}

func rateLimitConsumed(resp *http.Response) bool {
	//nolint:godox // wip
	// TODO: add other cases
	return resp.StatusCode != http.StatusNotModified
}
