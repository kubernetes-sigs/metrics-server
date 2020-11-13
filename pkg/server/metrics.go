// Copyright 2020 The Kubernetes Authors.
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

package server

import (
	"fmt"
	"time"

	"k8s.io/component-base/metrics"

	"sigs.k8s.io/metrics-server/pkg/api"
	"sigs.k8s.io/metrics-server/pkg/scraper"
	"sigs.k8s.io/metrics-server/pkg/storage"
)

// RegisterMetrics registers
func RegisterMetrics(r metrics.KubeRegistry, metricResolution time.Duration) error {
	// register metrics server components metrics
	err := RegisterServerMetrics(r.Register, metricResolution)
	if err != nil {
		return fmt.Errorf("unable to register server metrics: %v", err)
	}
	err = scraper.RegisterScraperMetrics(r.Register)
	if err != nil {
		return fmt.Errorf("unable to register scraper metrics: %v", err)
	}
	err = api.RegisterAPIMetrics(r.Register)
	if err != nil {
		return fmt.Errorf("unable to register API metrics: %v", err)
	}
	err = storage.RegisterStorageMetrics(r.Register)
	if err != nil {
		return fmt.Errorf("unable to register storage metrics: %v", err)
	}

	return nil
}
