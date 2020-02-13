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

package summary

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

var (
	// DefaultAddressTypePriority is the default node address type
	// priority list, as taken from the Kubernetes API server options.
	// In general, we prefer overrides to others, internal to external,
	// and DNS to IPs.
	DefaultAddressTypePriority = []corev1.NodeAddressType{
		// --override-hostname
		corev1.NodeHostName,

		// internal, preferring DNS if reported
		corev1.NodeInternalDNS,
		corev1.NodeInternalIP,

		// external, preferring DNS if reported
		corev1.NodeExternalDNS,
		corev1.NodeExternalIP,
	}
)

// NodeAddressResolver knows how to find the preferred connection
// address for a given node.
type NodeAddressResolver interface {
	// NodeAddress finds the preferred address to use to connect to
	// the given node.
	NodeAddress(node *corev1.Node) (address string, err error)
}

// prioNodeAddrResolver finds node addresses according to a list of
// priorities of types of addresses.
type prioNodeAddrResolver struct {
	addrTypePriority []corev1.NodeAddressType
}

func (r *prioNodeAddrResolver) NodeAddress(node *corev1.Node) (string, error) {
	// adapted from k8s.io/kubernetes/pkg/util/node
	for _, addrType := range r.addrTypePriority {
		for _, addr := range node.Status.Addresses {
			if addr.Type == addrType {
				return addr.Address, nil
			}
		}
	}

	return "", fmt.Errorf("node %s had no addresses that matched types %v", node.Name, r.addrTypePriority)
}

// NewPriorityNodeAddressResolver creates a new NodeAddressResolver that resolves
// addresses first based on a list of prioritized address types, then based on
// address order (first to last) within a particular address type.
func NewPriorityNodeAddressResolver(typePriority []corev1.NodeAddressType) NodeAddressResolver {
	return &prioNodeAddrResolver{
		addrTypePriority: typePriority,
	}
}
