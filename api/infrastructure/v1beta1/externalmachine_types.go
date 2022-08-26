/*
Copyright 2021.

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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ExternalMachineSpec defines the desired state of ExternalMachine
type ExternalMachineSpec struct {
	// ProviderID is the unique identifier as specified by the cloud provider.
	// +optional
	ProviderID string `json:"providerID,omitempty"`
}

// ExternalMachineStatus defines the observed state of ExternalMachine
type ExternalMachineStatus struct {
	// +optional
	Ready bool `json:"ready"`

	// Conditions defines current service state of the NodeletControlPlane.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// Addresses contains the IP and/or DNS addresses of the CoxEdge instances.
	// +optional
	Addresses []corev1.NodeAddress `json:"addresses,omitempty"`
}

// GetConditions returns the set of conditions for this object.
func (in *ExternalMachine) GetConditions() clusterv1.Conditions {
	return in.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (in *ExternalMachine) SetConditions(conditions clusterv1.Conditions) {
	in.Status.Conditions = conditions
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Machine to which this ExternalMachine belongs"
// +kubebuilder:printcolumn:name="Provider ID",type="string",JSONPath=".spec.providerID",description="The machine-specific provider identifier"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Machine infrastructure is ready for External instances"

// ExternalMachine is the Schema for the externalclusters API
type ExternalMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExternalMachineSpec   `json:"spec,omitempty"`
	Status ExternalMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ExternalMachineList contains a list of ExternalMachine
type ExternalMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExternalMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExternalMachine{}, &ExternalMachineList{})
}
