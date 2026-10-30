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
	"slices"
	"sort"

	v1 "k8s.io/api/core/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/metrics/pkg/apis/metrics"
)

func addPodMetricsToTable(table *metav1beta1.Table, pods ...metrics.PodMetrics) {
	// Columns must describe every resource reported for any item, not just the
	// first one: storage returns resources per item and may return an item
	// without any usage at all.
	var names []string
	for _, pod := range pods {
		for _, container := range pod.Containers {
			names = addResourceNames(names, container.Usage)
		}
	}
	sort.Strings(names)
	if len(pods) != 0 {
		table.ColumnDefinitions = resourceTableColumns(names)
	}

	usage := make(v1.ResourceList, 3)
	for i, pod := range pods {
		for k := range usage {
			delete(usage, k)
		}
		for _, container := range pod.Containers {
			for k, v := range container.Usage {
				u := usage[k]
				u.Add(v)
				usage[k] = u
			}
		}
		row := make([]any, 0, len(names)+1)
		row = append(row, pod.Name)
		for _, name := range names {
			v := usage[v1.ResourceName(name)]
			row = append(row, v.String())
		}
		row = append(row, pod.Window.Duration.String())
		table.Rows = append(table.Rows, metav1beta1.TableRow{
			Cells:  row,
			Object: runtime.RawExtension{Object: &pods[i]},
		})
	}
}

func addNodeMetricsToTable(table *metav1beta1.Table, nodes ...metrics.NodeMetrics) {
	var names []string
	for _, node := range nodes {
		names = addResourceNames(names, node.Usage)
	}
	sort.Strings(names)
	if len(nodes) != 0 {
		table.ColumnDefinitions = resourceTableColumns(names)
	}

	for i, node := range nodes {
		row := make([]any, 0, len(names)+1)
		row = append(row, node.Name)
		for _, name := range names {
			v := node.Usage[v1.ResourceName(name)]
			row = append(row, v.String())
		}
		row = append(row, node.Window.Duration.String())
		table.Rows = append(table.Rows, metav1beta1.TableRow{
			Cells:  row,
			Object: runtime.RawExtension{Object: &nodes[i]},
		})
	}
}

// addResourceNames appends the resource names of usage that names does not
// already contain.
func addResourceNames(names []string, usage v1.ResourceList) []string {
	for k := range usage {
		name := string(k)
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}

// resourceTableColumns describes one column per resource name, preceded by the
// item name and followed by the measurement window.
func resourceTableColumns(names []string) []metav1beta1.TableColumnDefinition {
	columns := []metav1beta1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "Name of the resource"},
	}
	for _, name := range names {
		columns = append(columns, metav1beta1.TableColumnDefinition{
			Name:   name,
			Type:   "string",
			Format: "quantity",
		})
	}
	return append(columns, metav1beta1.TableColumnDefinition{
		Name:   "Window",
		Type:   "string",
		Format: "duration",
	})
}
