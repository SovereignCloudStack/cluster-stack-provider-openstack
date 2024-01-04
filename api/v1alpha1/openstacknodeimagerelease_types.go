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
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1alpha7 "sigs.k8s.io/cluster-api-provider-openstack/api/v1alpha7"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OpenStackNodeImageReleaseSpec defines the desired state of OpenStackNodeImageRelease.
type OpenStackNodeImageReleaseSpec struct {
	// The name of the cloud to use from the clouds secret
	CloudName string `json:"cloudName"`
	// IdentityRef is a reference to a identity to be used when reconciling this cluster
	IdentityRef *apiv1alpha7.OpenStackIdentityReference `json:"identityRef"`
	// Image represents options used to upload an image
	Image *OpenStackNodeImage `json:"image"`
}

// OpenStackNodeImage defines image fields required for image upload.
type OpenStackNodeImage struct {
	URL string `json:"url" yaml:"url"`
	// CreateOpts represents options used to create an image.
	CreateOpts *CreateOpts `json:"createOpts" yaml:"createOpts"`
}

// CreateOpts represents options used to create an image.
// CreateOpts attributes are taken from github.com/gophercloud/gophercloud/openstack/imageservice/v2/images.
// Consider.
type CreateOpts struct {
	// Name is the name of the new image.
	Name string `json:"name" yaml:"name" required:"true"`
	// Id is the the image ID.
	ID string `json:"id,omitempty" yaml:"id,omitempty"`
	// Visibility defines who can see/use the image.
	Visibility *images.ImageVisibility `json:"visibility,omitempty" yaml:"visibility,omitempty"`
	// Hidden is whether the image is listed in default image list or not.
	Hidden *bool `json:"os_hidden,omitempty" yaml:"os_hidden,omitempty"` //nolint:tagliatelle // OS API expects snake case query attrs
	// Tags is a set of image tags.
	Tags []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	// ContainerFormat is the format of the
	// container. Valid values are ami, ari, aki, bare, and ovf.
	ContainerFormat string `json:"container_format,omitempty" yaml:"container_format,omitempty"` //nolint:tagliatelle // OS API expects snake case query attrs
	// DiskFormat is the format of the disk. If set,
	// valid values are ami, ari, aki, vhd, vmdk, raw, qcow2, vdi,
	// and iso.
	DiskFormat string `json:"disk_format,omitempty" yaml:"disk_format,omitempty"` //nolint:tagliatelle // OS API expects snake case query attrs
	// MinDisk is the amount of disk space in
	// GB that is required to boot the image.
	MinDisk int `json:"min_disk,omitempty" yaml:"min_disk,omitempty"` //nolint:tagliatelle // OS API expects snake case query attrs
	// MinRAM is the amount of RAM in MB that
	// is required to boot the image.
	MinRAM int `json:"min_ram,omitempty" yaml:"min_ram,omitempty"` //nolint:tagliatelle // OS API expects snake case query attrs
	// protected is whether the image is not deletable.
	Protected *bool `json:"protected,omitempty" yaml:"protected,omitempty"`
	// properties is a set of properties, if any, that
	// are associated with the image.
	Properties map[string]string `json:"-" yaml:"-"`
}

// OpenStackNodeImageReleaseStatus defines the observed state of OpenStackNodeImageRelease.
type OpenStackNodeImageReleaseStatus struct {
	// +optional
	// +kubebuilder:default:=false
	Ready bool `json:"ready,omitempty"`
	// Conditions defines current service state of the OpenStackNodeImageRelease.
	// +optional
	Conditions clusterv1beta1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of OpenStackNodeImageRelease"
//+kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"

// OpenStackNodeImageRelease is the Schema for the openstacknodeimagereleases API.
type OpenStackNodeImageRelease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackNodeImageReleaseSpec   `json:"spec,omitempty"`
	Status OpenStackNodeImageReleaseStatus `json:"status,omitempty"`
}

// GetConditions returns the observations of the operational state of the OpenStackNodeImageRelease resource.
func (r *OpenStackNodeImageRelease) GetConditions() clusterv1beta1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the OpenStackNodeImageRelease to the predescribed clusterv1.Conditions.
func (r *OpenStackNodeImageRelease) SetConditions(conditions clusterv1beta1.Conditions) {
	r.Status.Conditions = conditions
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
