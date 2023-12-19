/*
Copyright 2023 The Kubernetes Authors.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OpenStackClusterStackReleaseSpec defines the desired state of OpenStackClusterStackRelease.
type OpenStackClusterStackReleaseSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The name of the cloud to use from the clouds secret
	CloudName string `json:"cloudName"`

	// IdentityRef is a reference to a identity to be used when reconciling this cluster
	IdentityRef OpenStackIdentityReference `json:"identityRef,omitempty"`
}

// OpenStackIdentityReference is a reference to an infrastructure
// provider identity to be used to provision cluster resources.
type OpenStackIdentityReference struct {
	// Kind of the identity. Must be supported by the infrastructure
	// provider and may be either cluster or namespace-scoped.
	// +kubebuilder:validation:MinLength=1
	Kind string `json:"kind"`

	// Name of the infrastructure identity to be used.
	// Must be either a cluster-scoped resource, or namespaced-scoped
	// resource the same namespace as the resource(s) being provisioned.
	Name string `json:"name"`
}

// OpenStackClusterStackReleaseStatus defines the observed state of OpenStackClusterStackRelease.
type OpenStackClusterStackReleaseStatus struct {
	// +optional
	// +kubebuilder:default:=false
	Ready bool `json:"ready,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"

// OpenStackClusterStackRelease is the Schema for the openstackclusterstackreleases API.
type OpenStackClusterStackRelease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackClusterStackReleaseSpec   `json:"spec,omitempty"`
	Status OpenStackClusterStackReleaseStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackClusterStackReleaseList contains a list of OpenStackClusterStackRelease.
type OpenStackClusterStackReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackClusterStackRelease `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackClusterStackRelease{}, &OpenStackClusterStackReleaseList{})
}
