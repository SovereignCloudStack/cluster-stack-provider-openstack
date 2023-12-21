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
	apiv1alpha7 "sigs.k8s.io/cluster-api-provider-openstack/api/v1alpha7"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OpenStackNodeImageReleaseSpec defines the desired state of OpenStackNodeImageRelease.
type OpenStackNodeImageReleaseSpec struct {
	// The name of the node image
	Name string `json:"name"`
	// The URL of the node image
	URL string `json:"url"`
	// The name of the cloud to use from the clouds secret
	CloudName string `json:"cloudName"`
	// IdentityRef is a reference to a identity to be used when reconciling this cluster
	IdentityRef *apiv1alpha7.OpenStackIdentityReference `json:"identityRef,omitempty"`
}

// OpenStackNodeImageReleaseStatus defines the observed state of OpenStackNodeImageRelease.
type OpenStackNodeImageReleaseStatus struct {
	// +optional
	// +kubebuilder:default:=false
	Ready bool `json:"ready,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// OpenStackNodeImageRelease is the Schema for the openstacknodeimagereleases API.
type OpenStackNodeImageRelease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackNodeImageReleaseSpec   `json:"spec,omitempty"`
	Status OpenStackNodeImageReleaseStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackNodeImageReleaseList contains a list of OpenStackNodeImageRelease.
type OpenStackNodeImageReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackNodeImageRelease `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackNodeImageRelease{}, &OpenStackNodeImageReleaseList{})
}
