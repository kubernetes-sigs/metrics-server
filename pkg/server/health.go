// Copyright 2021 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/server/healthz"
)

type cacheSyncWaiter interface {
	WaitForCacheSync(stopCh <-chan struct{}) map[schema.GroupVersionResource]bool
}

type metadataInformerSync struct {
	name            string
	cacheSyncWaiter cacheSyncWaiter
}

var _ healthz.HealthChecker = &metadataInformerSync{}

// MetadataInformerSyncHealthz returns a new HealthChecker that will pass only if all informers in the given cacheSyncWaiter sync.
func MetadataInformerSyncHealthz(name string, cacheSyncWaiter cacheSyncWaiter) healthz.HealthChecker {
	return &metadataInformerSync{
		name:            name,
		cacheSyncWaiter: cacheSyncWaiter,
	}
}

func (i *metadataInformerSync) Name() string {
	return i.name
}

func (i *metadataInformerSync) Check(_ *http.Request) error {
	stopCh := make(chan struct{})
	// Close stopCh to force checking if informers are synced now.
	close(stopCh)

	informersByStarted := make(map[bool][]string)
	for informerType, started := range i.cacheSyncWaiter.WaitForCacheSync(stopCh) {
		informersByStarted[started] = append(informersByStarted[started], informerType.String())
	}

	if notStarted := informersByStarted[false]; len(notStarted) > 0 {
		return fmt.Errorf("%d informers not started yet: %v", len(notStarted), notStarted)
	}
	return nil
}
