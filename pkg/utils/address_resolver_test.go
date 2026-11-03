// Copyright 2026 The Kubernetes Authors.
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

package utils

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestNodeAddress(t *testing.T) {
	for _, tc := range []struct {
		name      string
		priority  []corev1.NodeAddressType
		addresses []corev1.NodeAddress
		expected  string
		expectErr bool
	}{
		{
			name:     "respects address type priority",
			priority: []corev1.NodeAddressType{corev1.NodeExternalIP, corev1.NodeInternalIP},
			addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.2"},
				{Type: corev1.NodeExternalIP, Address: "198.51.100.9"},
			},
			expected: "198.51.100.9",
		},
		{
			name:     "prefers private internal ip when multiple internal ips are present",
			priority: []corev1.NodeAddressType{corev1.NodeInternalIP, corev1.NodeExternalIP},
			addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "62.238.34.122"},
				{Type: corev1.NodeInternalIP, Address: "10.42.0.7"},
				{Type: corev1.NodeExternalIP, Address: "203.0.113.8"},
			},
			expected: "10.42.0.7",
		},
		{
			name:     "falls back to first internal ip when no private internal ip exists",
			priority: []corev1.NodeAddressType{corev1.NodeInternalIP, corev1.NodeExternalIP},
			addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "62.238.34.122"},
				{Type: corev1.NodeInternalIP, Address: "198.51.100.10"},
			},
			expected: "62.238.34.122",
		},
		{
			name:      "returns error when no configured type exists on node",
			priority:  []corev1.NodeAddressType{corev1.NodeInternalDNS},
			addresses: []corev1.NodeAddress{{Type: corev1.NodeHostName, Address: "node-a"}},
			expectErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver := NewPriorityNodeAddressResolver(tc.priority)
			node := &corev1.Node{Status: corev1.NodeStatus{Addresses: tc.addresses}}
			address, err := resolver.NodeAddress(node)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if address != tc.expected {
				t.Fatalf("unexpected address, want %q, got %q", tc.expected, address)
			}
		})
	}
}
