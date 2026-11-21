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
	"sort"

	v1 "k8s.io/api/core/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/metrics/pkg/apis/metrics"
)

func addPodMetricsToTable(table *metav1beta1.Table, pods ...metrics.PodMetrics) {
	if len(pods) == 0 {
		return
	}
	usages := make([]v1.ResourceList, len(pods))
	nameSet := make(map[string]struct{}, 3)
	for i, pod := range pods {
		usage := make(v1.ResourceList, 3)
		for _, container := range pod.Containers {
			for k, v := range container.Usage {
				u := usage[k]
				u.Add(v)
				usage[k] = u
			}
		}
		usages[i] = usage
		for k := range usage {
			nameSet[string(k)] = struct{}{}
		}
	}
	names := make([]string, 0, len(nameSet))
	for k := range nameSet {
		names = append(names, k)
	}
	sort.Strings(names)

	table.ColumnDefinitions = []metav1beta1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "Name of the resource"},
	}
	for _, name := range names {
		table.ColumnDefinitions = append(table.ColumnDefinitions, metav1beta1.TableColumnDefinition{
			Name:   name,
			Type:   "string",
			Format: "quantity",
		})
	}
	table.ColumnDefinitions = append(table.ColumnDefinitions, metav1beta1.TableColumnDefinition{
		Name:   "Window",
		Type:   "string",
		Format: "duration",
	})

	for i, pod := range pods {
		row := make([]any, 0, len(names)+2)
		row = append(row, pod.Name)
		for _, name := range names {
			if v, ok := usages[i][v1.ResourceName(name)]; ok {
				row = append(row, v.String())
			} else {
				row = append(row, "") // not reported: leave blank rather than a false 0
			}
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
	for i, node := range nodes {
		if names == nil {
			for k := range node.Usage {
				names = append(names, string(k))
			}
			sort.Strings(names)

			table.ColumnDefinitions = []metav1beta1.TableColumnDefinition{
				{Name: "Name", Type: "string", Format: "name", Description: "Name of the resource"},
			}
			for _, name := range names {
				table.ColumnDefinitions = append(table.ColumnDefinitions, metav1beta1.TableColumnDefinition{
					Name:   name,
					Type:   "string",
					Format: "quantity",
				})
			}
			table.ColumnDefinitions = append(table.ColumnDefinitions, metav1beta1.TableColumnDefinition{
				Name:   "Window",
				Type:   "string",
				Format: "duration",
			})
		}
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
