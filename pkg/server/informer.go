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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/rest"
)

const (
	// we should never need to resync, since we're not worried about missing events,
	// and resync is actually for regular interval-based reconciliation these days,
	// so set the default resync interval to 0
	defaultResync = 0
)

func informerFactory(rest *rest.Config) (informers.SharedInformerFactory, error) {
	client, err := kubernetes.NewForConfig(rest)
	if err != nil {
		return nil, fmt.Errorf("unable to construct lister client: %v", err)
	}
	return informers.NewSharedInformerFactory(client, defaultResync), nil
}

func runningPodMetadataInformer(rest *rest.Config) (metadatainformer.SharedInformerFactory, error) {
	client, err := metadata.NewForConfig(rest)
	if err != nil {
		return nil, fmt.Errorf("unable to construct lister client: %v", err)
	}
	return metadatainformer.NewFilteredSharedInformerFactory(client, defaultResync, corev1.NamespaceAll, func(options *metav1.ListOptions) {
		options.FieldSelector = "status.phase=Running"
	}), nil
}
