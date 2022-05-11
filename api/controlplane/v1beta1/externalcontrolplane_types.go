package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExternalControlPlaneSpec defines the desired state of ExternalControlPlane.
type ExternalControlPlaneSpec struct {
}

// ExternalControlPlaneStatus defines the observed state of ExternalControlPlane.
type ExternalControlPlaneStatus struct {
	// Version represents the minimum Kubernetes version for the control plane machines
	// in the cluster.
	// +optional
	Version *string `json:"version,omitempty"`

	// Initialized denotes whether or not the control plane has the
	// uploaded external-config configmap.
	// +optional
	Initialized bool `json:"initialized"`

	// Ready denotes that the ExternalControlPlane API Server is ready to
	// receive requests.
	// +optional
	Ready bool `json:"ready"`

	// FailureReason indicates that there is a terminal problem reconciling the
	// state, and will be set to a token value suitable for
	// programmatic interpretation.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// ErrorMessage indicates that there is a terminal problem reconciling the
	// state, and will be set to a descriptive error message.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=externalcontrolplanes,shortName=ncp,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.selector
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels['cluster\\.x-k8s\\.io/cluster-name']",description="Cluster"
// +kubebuilder:printcolumn:name="Initialized",type=boolean,JSONPath=".status.initialized",description="This denotes whether or not the control plane has the uploaded external-config configmap"
// +kubebuilder:printcolumn:name="API Server Available",type=boolean,JSONPath=".status.ready",description="ExternalControlPlane API Server is ready to receive requests"
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=".status.replicas",description="Total number of non-terminated machines targeted by this control plane"
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=".status.version",description="Kubernetes version associated with this control plane"

// ExternalControlPlane is the Schema for the ExternalControlPlane API.
type ExternalControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExternalControlPlaneSpec   `json:"spec,omitempty"`
	Status ExternalControlPlaneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ExternalControlPlaneList contains a list of ExternalControlPlane.
type ExternalControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExternalControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExternalControlPlane{}, &ExternalControlPlaneList{})
}
