// Copyright 2021 The Kubernetes Authors.
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

package api

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apiserver/pkg/registry/generic"
)

func filterNodes(nodes []*v1.Node, selector fields.Selector) []*v1.Node {
	newNodes := make([]*v1.Node, 0, len(nodes))
	fields := make(fields.Set, 2)
	for _, node := range nodes {
		for k := range fields {
			delete(fields, k)
		}
		fieldsSet := generic.AddObjectMetaFieldsSet(fields, &node.ObjectMeta, false)
		if !selector.Matches(fieldsSet) {
			continue
		}
		newNodes = append(newNodes, node)
	}
	return newNodes
}

func filterPods(pods []*v1.Pod, selector fields.Selector) []*v1.Pod {
	newPods := make([]*v1.Pod, 0, len(pods))
	fields := make(fields.Set, 2)
	for _, pod := range pods {
		for k := range fields {
			delete(fields, k)
		}
		fieldsSet := generic.AddObjectMetaFieldsSet(fields, &pod.ObjectMeta, true)
		if !selector.Matches(fieldsSet) {
			continue
		}
		newPods = append(newPods, pod)
	}
	return newPods
}
