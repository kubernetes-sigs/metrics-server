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
	"net/http"

	apimetrics "k8s.io/apiserver/pkg/endpoints/metrics"
	"k8s.io/apiserver/pkg/server/mux"
	etcd3metrics "k8s.io/apiserver/pkg/storage/etcd3/metrics"
	flowcontrolmetrics "k8s.io/apiserver/pkg/util/flowcontrol/metrics"
	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
)

// DefaultMetrics installs the default prometheus metrics handler
type DefaultMetrics struct {
	registry metrics.Gatherer
}

// Install adds the DefaultMetrics handler
func (m DefaultMetrics) Install(c *mux.PathRecorderMux) {
	register()
	c.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
		legacyregistry.Handler().ServeHTTP(w, req)
		metrics.HandlerFor(m.registry, metrics.HandlerOpts{}).ServeHTTP(w, req)
	})
}

// register apiserver and etcd metrics
func register() {
	apimetrics.Register()
	etcd3metrics.Register()
	flowcontrolmetrics.Register()
}
