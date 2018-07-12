// Copyright 2018 The Kubernetes Authors.
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

package fake

import (
	"context"

	"github.com/kubernetes-incubator/metrics-server/pkg/sources"
)

// StaticSourceProvider is a fake sources.MetricSourceProvider that returns
// metrics from a static list.
type StaticSourceProvider []sources.MetricSource

func (p StaticSourceProvider) GetMetricSources() ([]sources.MetricSource, error) { return p, nil }

// FunctionSource is a sources.MetricSource that calls a function to
// return the given data points
type FunctionSource struct {
	SourceName    string
	GenerateBatch CollectFunc
}

func (f *FunctionSource) Name() string {
	return f.SourceName
}

func (f *FunctionSource) Collect(ctx context.Context) (*sources.MetricsBatch, error) {
	return f.GenerateBatch(ctx)
}

// CollectFunc is the function signature of FunctionSource#GenerateBatch,
// and knows how to generate a MetricsBatch.
type CollectFunc func(context.Context) (*sources.MetricsBatch, error)
