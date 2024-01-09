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

// OpenStackClusterStackReleaseTemplateSpec defines the desired state of OpenStackClusterStackReleaseTemplate.
type OpenStackClusterStackReleaseTemplateSpec struct {
	Template OpenStackClusterStackReleaseTemplateResource `json:"template"`
}

// OpenStackClusterStackReleaseTemplateResource describes the data needed to create a OpenStackClusterStackRelease from a template.
type OpenStackClusterStackReleaseTemplateResource struct {
	Spec OpenStackClusterStackReleaseSpec `json:"spec"`
}

// OpenStackClusterStackReleaseTemplateStatus defines the observed state of OpenStackClusterStackReleaseTemplate.
type OpenStackClusterStackReleaseTemplateStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=oscsrt
//+kubebuilder:subresource:status

// OpenStackClusterStackReleaseTemplate is the Schema for the openstackclusterstackreleasetemplates API.
type OpenStackClusterStackReleaseTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackClusterStackReleaseTemplateSpec   `json:"spec,omitempty"`
	Status OpenStackClusterStackReleaseTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackClusterStackReleaseTemplateList contains a list of OpenStackClusterStackReleaseTemplate.
type OpenStackClusterStackReleaseTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackClusterStackReleaseTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackClusterStackReleaseTemplate{}, &OpenStackClusterStackReleaseTemplateList{})
}
