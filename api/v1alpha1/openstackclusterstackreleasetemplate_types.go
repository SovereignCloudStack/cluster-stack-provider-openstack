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

// OpenstackClusterStackReleaseTemplateSpec defines the desired state of OpenstackClusterStackReleaseTemplate.
type OpenstackClusterStackReleaseTemplateSpec struct {
	Template OpenstackClusterStackReleaseTemplateResource `json:"template"`
}

// OpenstackClusterStackReleaseTemplateResource describes the data needed to create a OpenstackClusterStackRelease from a template.
type OpenstackClusterStackReleaseTemplateResource struct {
	Spec OpenstackClusterStackReleaseSpec `json:"spec"`
}

// OpenstackClusterStackReleaseTemplateStatus defines the observed state of OpenstackClusterStackReleaseTemplate.
type OpenstackClusterStackReleaseTemplateStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// OpenstackClusterStackReleaseTemplate is the Schema for the openstackclusterstackreleasetemplates API.
type OpenstackClusterStackReleaseTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenstackClusterStackReleaseTemplateSpec   `json:"spec,omitempty"`
	Status OpenstackClusterStackReleaseTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenstackClusterStackReleaseTemplateList contains a list of OpenstackClusterStackReleaseTemplate.
type OpenstackClusterStackReleaseTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenstackClusterStackReleaseTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenstackClusterStackReleaseTemplate{}, &OpenstackClusterStackReleaseTemplateList{})
}
