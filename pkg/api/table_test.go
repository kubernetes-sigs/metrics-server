/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"testing"

	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
)

func TestNodeList_ConvertToTable(t *testing.T) {
	// setup
	r := NewTestNodeStorage(nil)

	// execute
	got, err := r.List(genericapirequest.NewContext(), nil)

	// assert
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	res, err := r.ConvertToTable(genericapirequest.NewContext(), got, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(res.Rows) != 3 ||
		res.ColumnDefinitions[1].Name != "res1" || res.ColumnDefinitions[2].Name != "Window" ||
		res.Rows[0].Cells[0] != "node1" ||
		res.Rows[0].Cells[1] != "10m" ||
		res.Rows[0].Cells[2] != "1µs" ||
		res.Rows[1].Cells[0] != "node2" ||
		res.Rows[1].Cells[1] != "5Mi" ||
		res.Rows[1].Cells[2] != "2µs" ||
		res.Rows[2].Cells[0] != "node3" ||
		res.Rows[2].Cells[1] != "1" ||
		res.Rows[2].Cells[2] != "3µs" {
		t.Errorf("Got unexpected object: %+v", res)
	}
}

func TestPodList_ConvertToTable(t *testing.T) {
	// setup
	r := NewPodTestStorage(nil)

	// execute
	got, err := r.List(genericapirequest.NewContext(), nil)

	// assert
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	res, err := r.ConvertToTable(genericapirequest.NewContext(), got, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(res.Rows) != 3 ||
		res.ColumnDefinitions[1].Name != "cpu" || res.ColumnDefinitions[2].Name != "memory" || res.ColumnDefinitions[3].Name != "Window" ||
		res.Rows[0].Cells[0] != "pod1" ||
		res.Rows[0].Cells[1] != "10m" ||
		res.Rows[0].Cells[2] != "5Mi" ||
		res.Rows[0].Cells[3] != "1µs" ||
		res.Rows[1].Cells[0] != "pod2" ||
		res.Rows[1].Cells[1] != "20m" ||
		res.Rows[1].Cells[2] != "15Mi" ||
		res.Rows[1].Cells[3] != "2µs" ||
		res.Rows[2].Cells[0] != "pod3" ||
		res.Rows[2].Cells[1] != "20m" ||
		res.Rows[2].Cells[2] != "25Mi" ||
		res.Rows[2].Cells[3] != "3µs" {
		t.Errorf("Got unexpected object: %+v", res)
	}
}
