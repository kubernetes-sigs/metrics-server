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

package util

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"sort"
	"strings"
	"time"
)

var labelSeperator string

// Concatenates a map of labels into a Seperator-seperated key:value pairs.
func LabelsToString(labels map[string]string) string {
	output := make([]string, 0, len(labels))
	for key, value := range labels {
		output = append(output, fmt.Sprintf("%s:%s", key, value))
	}

	// Sort to produce a stable output.
	sort.Strings(output)
	return strings.Join(output, labelSeperator)
}

func SetLabelSeperator(seperator string) {
	labelSeperator = seperator
}

func GetNodeLister(restClient rest.Interface) (v1listers.NodeLister, *cache.Reflector, error) {
	lw := cache.NewListWatchFromClient(restClient, "nodes", corev1.NamespaceAll, fields.Everything())
	store := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	nodeLister := v1listers.NewNodeLister(store)
	reflector := cache.NewReflector(lw, &corev1.Node{}, store, time.Hour)
	go reflector.Run(wait.NeverStop)

	return nodeLister, reflector, nil
}
