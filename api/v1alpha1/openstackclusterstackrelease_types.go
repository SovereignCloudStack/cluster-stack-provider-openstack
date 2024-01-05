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
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OpenStackClusterStackReleaseSpec defines the desired state of OpenStackClusterStackRelease.
type OpenStackClusterStackReleaseSpec struct {
	// The name of the cloud to use from the clouds secret
	CloudName string `json:"cloudName"`
	// IdentityRef is a reference to a identity to be used when reconciling this cluster
	IdentityRef *apiv1alpha7.OpenStackIdentityReference `json:"identityRef"`
}

// OpenStackClusterStackReleaseStatus defines the observed state of OpenStackClusterStackRelease.
type OpenStackClusterStackReleaseStatus struct {
	// +optional
	// +kubebuilder:default:=false
	Ready bool `json:"ready,omitempty"`
	// Conditions defines current service state of the OpenStackClusterStackRelease.
	// +optional
	Conditions clusterv1beta1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=oscsr
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of OpenStackClusterStackRelease"
//+kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"

// OpenStackClusterStackRelease is the Schema for the openstackclusterstackreleases API.
type OpenStackClusterStackRelease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackClusterStackReleaseSpec   `json:"spec,omitempty"`
	Status OpenStackClusterStackReleaseStatus `json:"status,omitempty"`
}

// GetConditions returns the observations of the operational state of the OpenStackClusterStackRelease resource.
func (r *OpenStackClusterStackRelease) GetConditions() clusterv1beta1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the OpenStackClusterStackRelease to the predescribed clusterv1.Conditions.
func (r *OpenStackClusterStackRelease) SetConditions(conditions clusterv1beta1.Conditions) {
	r.Status.Conditions = conditions
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
