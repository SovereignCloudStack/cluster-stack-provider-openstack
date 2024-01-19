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

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv1alpha1 "github.com/sovereignCloudStack/cluster-stack-provider-openstack/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	capoapiv1alpha7 "sigs.k8s.io/cluster-api-provider-openstack/api/v1alpha7"
)

const (
	timeout  = time.Second * 2
	interval = 100 * time.Millisecond
)

func TestCutOpenStackClusterStackReleaseVersionFromReleaseTag(t *testing.T) {
	// correct release tag
	releaseTag := "openstack-ferrol-1-27-v2"
	nameWithoutVersion, err := cutOpenStackClusterStackReleaseVersionFromReleaseTag(releaseTag)

	assert.NoError(t, err)
	assert.Equal(t, nameWithoutVersion, "openstack-ferrol-1-27")

	// incorrect release tag
	releaseTag = "openstack-ferrol-1-27"
	nameWithoutVersion, err = cutOpenStackClusterStackReleaseVersionFromReleaseTag(releaseTag)

	assert.Error(t, err)
	assert.Empty(t, nameWithoutVersion)
}

func TestGenerateOwnerReference(t *testing.T) {
	oscsr := &apiv1alpha1.OpenStackClusterStackRelease{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiv1alpha1.GroupVersion.String(),
			Kind:       "OpenStackClusterStackRelease",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openstack-ferrol-1-27-v1",
			Namespace: "cluster",
			UID:       "fb686e33-01a6-42c9-a210-2c26ec8cb331",
		},
		Spec: apiv1alpha1.OpenStackClusterStackReleaseSpec{
			CloudName: "openstack",
			IdentityRef: &capoapiv1alpha7.OpenStackIdentityReference{
				Kind: "Secret",
				Name: "supersecret",
			},
		},
	}

	ownerRef := generateOwnerReference(oscsr)

	assert.Equal(t, ownerRef, &metav1.OwnerReference{
		APIVersion: oscsr.APIVersion,
		Kind:       oscsr.Kind,
		Name:       oscsr.Name,
		UID:        oscsr.UID,
	})
}

func TestMatchOwnerReference(t *testing.T) {
	// wrong APIVersion
	oscsr1 := &apiv1alpha1.OpenStackClusterStackRelease{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiv1alpha1.GroupVersion.String() + "/" + apiv1alpha1.GroupVersion.Version,
			Kind:       "OpenStackClusterStackRelease",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "openstack-ferrol-1-27-v1",
		},
		Spec: apiv1alpha1.OpenStackClusterStackReleaseSpec{
			CloudName: "openstack",
			IdentityRef: &capoapiv1alpha7.OpenStackIdentityReference{
				Kind: "Secret",
				Name: "supersecret1",
			},
		},
	}
	assert.False(t, matchOwnerReference(generateOwnerReference(oscsr1), oscsr1))

	// match owner reference
	oscsr2 := &apiv1alpha1.OpenStackClusterStackRelease{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiv1alpha1.GroupVersion.String(),
			Kind:       "OpenStackClusterStackRelease",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "openstack-ferrol-1-27-v2",
		},
		Spec: apiv1alpha1.OpenStackClusterStackReleaseSpec{
			CloudName: "openstack",
			IdentityRef: &capoapiv1alpha7.OpenStackIdentityReference{
				Kind: "Secret",
				Name: "supersecret2",
			},
		},
	}
	assert.True(t, matchOwnerReference(generateOwnerReference(oscsr2), oscsr2))

	// name is different
	ownerRef := &metav1.OwnerReference{
		APIVersion: oscsr2.APIVersion,
		Kind:       oscsr2.Kind,
		Name:       "openstack-ferrol-1-27-v3",
		UID:        oscsr2.UID,
	}
	assert.False(t, matchOwnerReference(ownerRef, oscsr2))
}

func TestGetNodeImagesFromLocal(t *testing.T) {
	// wrong path
	nodeImages, err := getNodeImagesFromLocal("../../test/releases/cluster-stacks/openstack-ferrol-1-27-4v")
	assert.Error(t, err)
	assert.Nil(t, nodeImages)

	// wrong node-images file
	nodeImages, err = getNodeImagesFromLocal("../../test/releases/cluster-stacks/openstack-ferrol-1-27-v4-wrong-node-images")
	assert.Error(t, err)
	assert.Nil(t, nodeImages)

	// correct node-images file
	nodeImages, err = getNodeImagesFromLocal("../../test/releases/cluster-stacks/openstack-ferrol-1-27-v4")
	var visibility images.ImageVisibility = "shared"
	assert.NoError(t, err)
	assert.Equal(t, nodeImages, &NodeImages{
		[]*apiv1alpha1.OpenStackNodeImage{
			{
				URL: "https://swift.services.a.regiocloud.tech/swift/v1/AUTH_b182637428444b9aa302bb8d5a5a418c/openstack-k8s-capi-images/ubuntu-2204-kube-v1.27/ubuntu-2204-kube-v1.27.8.qcow2",
				CreateOpts: &apiv1alpha1.CreateOpts{
					Name:            "ubuntu-capi-image-v1.27.8",
					DiskFormat:      "qcow2",
					ContainerFormat: "bare",
					Visibility:      &visibility,
				},
			},
		},
	})
}

var _ = Describe("OpenStackClusterStackRelease controller", func() {
	Context("OpenStackClusterStackRelease controller test", func() {
		const openstackclusterstackreleasename = "test-ocsr"

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      openstackclusterstackreleasename,
				Namespace: openstackclusterstackreleasename,
			},
		}

		typeNamespaceName := types.NamespacedName{Name: openstackclusterstackreleasename, Namespace: openstackclusterstackreleasename}

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)
		})

		It("should successfully create the OpenStackClusterStackRelease custom resource", func() {
			By("Creating the custom resource for the Kind OpenStackClusterStackRelease")
			openstackclusterstackrelease := &apiv1alpha1.OpenStackClusterStackRelease{}
			err := k8sClient.Get(ctx, typeNamespaceName, openstackclusterstackrelease)
			if err != nil && apierrors.IsNotFound(err) {
				openstackclusterstackrelease := &apiv1alpha1.OpenStackClusterStackRelease{
					ObjectMeta: metav1.ObjectMeta{
						Name:      openstackclusterstackreleasename,
						Namespace: namespace.Name,
					},
					Spec: apiv1alpha1.OpenStackClusterStackReleaseSpec{
						CloudName: "openstack",
						IdentityRef: &capoapiv1alpha7.OpenStackIdentityReference{
							Kind: "Secret",
							Name: "supersecret",
						},
					},
				}

				err = k8sClient.Create(ctx, openstackclusterstackrelease)
				Expect(err).To(Not(HaveOccurred()))
			}

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &apiv1alpha1.OpenStackClusterStackRelease{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, timeout, interval).Should(Succeed())
		})
	})
})
