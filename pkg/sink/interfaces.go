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

package sink

import (
	"github.com/kubernetes-incubator/metrics-server/pkg/sources"
)

// MetricSink knows how to receive metrics batches from a source.
type MetricSink interface {
	// Receive ingests a new batch of metrics.
	Receive(*sources.MetricsBatch) error
}
