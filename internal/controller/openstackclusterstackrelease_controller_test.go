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
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	githubmocks "github.com/SovereignCloudStack/cluster-stack-operator/pkg/github/client/mocks"
	apiv1alpha1 "github.com/SovereignCloudStack/cluster-stack-provider-openstack/api/v1alpha1"
	"github.com/google/go-github/v52/github"
	"github.com/gophercloud/gophercloud/v2/openstack/imageservice/v2/images"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	capoapiv1alpha7 "sigs.k8s.io/cluster-api-provider-openstack/api/v1alpha7"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	timeout  = time.Second * 2
	interval = 100 * time.Millisecond
)

func TestCutOpenStackClusterStackReleaseVersionFromReleaseTag(t *testing.T) {
	tests := []struct {
		releaseTag string
		expected   string
		incorrect  bool
	}{
		{
			"openstack-ferrol-1-27-v2", // stable channel
			"openstack-ferrol-1-27",
			false,
		},
		{
			"openstack-ferrol-1-27-v0-sha-a3baa3a", // custom channel (hash mode)
			"openstack-ferrol-1-27",
			false,
		},
		{
			"docker-ferrol-1-26-v1-alpha-0", // alpha channel
			"docker-ferrol-1-26",
			false,
		},
		{
			"openstack-ferrol-1-27", // incorrect tag
			"",
			true,
		},
		{
			"docker-ferrol-1.26", // incorrect tag
			"",
			true,
		},
	}
	for _, test := range tests {
		nameWithoutVersion, err := cutOpenStackClusterStackReleaseVersionFromReleaseTag(test.releaseTag)
		if test.incorrect {
			assert.Error(t, err)
			assert.EqualError(t, err, fmt.Sprintf("invalid release tag %s", test.releaseTag))
		} else {
			assert.NoError(t, err)
		}

		assert.Equal(t, nameWithoutVersion, test.expected)
	}
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
	wrongPath := "../../test/releases/cluster-stacks/openstack-ferrol-1-27-4v"
	nodeImages, err := getNodeImagesFromLocal(wrongPath)
	assert.Error(t, err)
	assert.ErrorContains(t, err, fmt.Sprintf("failed to read node-images file %s/node-images.yaml", wrongPath))
	assert.Nil(t, nodeImages)

	// wrong node-images file
	nodeImages, err = getNodeImagesFromLocal("../../test/releases/cluster-stacks/openstack-ferrol-1-27-v4-wrong-node-images")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to unmarshal node-images")
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

func TestDownloadReleaseAssets(t *testing.T) {
	ctx := context.TODO()
	releaseTag := "v1.0.0"
	downloadPath := "/tmp/download"
	assetlist := []string{metadataFileName, nodeImagesFileName}
	mockGitHubClient := githubmocks.NewClient(t)
	mockHTTPResponse := &http.Response{
		StatusCode: http.StatusOK,
	}
	mockResponse := &github.Response{
		Response: mockHTTPResponse,
	}
	mockRepoRelease := &github.RepositoryRelease{
		Name: github.String("test-release-name"),
	}

	mockGitHubClient.On("GetReleaseByTag", ctx, releaseTag).Return(mockRepoRelease, mockResponse, nil)
	repoRelease, resp, err := mockGitHubClient.GetReleaseByTag(ctx, releaseTag)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	mockGitHubClient.On("DownloadReleaseAssets", ctx, repoRelease, downloadPath, assetlist).Return(nil)

	err = downloadReleaseAssets(ctx, releaseTag, downloadPath, mockGitHubClient)

	assert.NoError(t, err)
}

func TestDownloadReleaseAssetsFailedToDownload(t *testing.T) {
	ctx := context.TODO()
	releaseTag := "v1.0.0"
	mockGitHubClient := githubmocks.NewClient(t)
	downloadPath := "/tmp/download"
	assetlist := []string{metadataFileName, nodeImagesFileName}
	mockHTTPResponse := &http.Response{
		StatusCode: http.StatusOK,
	}
	mockResponse := &github.Response{
		Response: mockHTTPResponse,
	}
	mockRepoRelease := &github.RepositoryRelease{
		Name: github.String("test-release-name"),
	}

	mockGitHubClient.On("GetReleaseByTag", ctx, releaseTag).Return(mockRepoRelease, mockResponse, nil)
	repoRelease, resp, err := mockGitHubClient.GetReleaseByTag(ctx, releaseTag)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	mockGitHubClient.On("DownloadReleaseAssets", ctx, repoRelease, downloadPath, assetlist).Return(errors.New("failed to download release assets"))
	err = downloadReleaseAssets(ctx, releaseTag, downloadPath, mockGitHubClient)

	assert.ErrorContains(t, err, "failed to download release assets")
}

func TestGetOwnedOpenStackNodeImageReleases(t *testing.T) {
	scheme := runtime.NewScheme()
	err := apiv1alpha1.AddToScheme(scheme)
	assert.NoError(t, err)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	openstackclusterstackrelease := &apiv1alpha1.OpenStackClusterStackRelease{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "clusterstack.x-k8s.io/v1alpha1",
			Kind:       "OpenStackClusterStackRelease",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-release",
			Namespace: "test-namespace",
		},
		Spec: apiv1alpha1.OpenStackClusterStackReleaseSpec{
			IdentityRef: &capoapiv1alpha7.OpenStackIdentityReference{
				Kind: "Secret",
				Name: "supersecret",
			},
		},
	}
	assert.NoError(t, client.Create(context.TODO(), openstackclusterstackrelease))

	openstackNodeImageRelease := &apiv1alpha1.OpenStackNodeImageRelease{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "clusterstack.x-k8s.io/v1alpha1",
			Kind:       "OpenStackNodeImageRelease",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node-image-release",
			Namespace: "test-namespace",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: openstackclusterstackrelease.APIVersion,
					Kind:       openstackclusterstackrelease.Kind,
					Name:       openstackclusterstackrelease.Name,
					UID:        openstackclusterstackrelease.UID,
				},
			},
		},
	}

	assert.NoError(t, client.Create(context.TODO(), openstackNodeImageRelease))

	r := &OpenStackClusterStackReleaseReconciler{
		Client: client,
	}

	ownedOpenStackNodeImageReleases, err := r.getOwnedOpenStackNodeImageReleases(context.TODO(), openstackclusterstackrelease)

	assert.NoError(t, err)
	assert.NotEmpty(t, ownedOpenStackNodeImageReleases)
	assert.Contains(t, ownedOpenStackNodeImageReleases, openstackNodeImageRelease)
}

func TestCreateOpenStackNodeImageRelease(t *testing.T) {
	scheme := runtime.NewScheme()
	err := apiv1alpha1.AddToScheme(scheme)
	assert.NoError(t, err)
	err = corev1.AddToScheme(scheme)
	assert.NoError(t, err)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	openstackclusterstackrelease := &apiv1alpha1.OpenStackClusterStackRelease{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "clusterstack.x-k8s.io/v1alpha1",
			Kind:       "OpenStackClusterStackRelease",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-release",
			Namespace: "test-namespace",
		},
		Spec: apiv1alpha1.OpenStackClusterStackReleaseSpec{
			IdentityRef: &capoapiv1alpha7.OpenStackIdentityReference{
				Kind: "Secret",
				Name: "supersecret",
			},
		},
	}
	assert.NoError(t, client.Create(context.TODO(), openstackclusterstackrelease))

	openStackNodeImage := &apiv1alpha1.OpenStackNodeImage{
		URL: "test-url",
		CreateOpts: &apiv1alpha1.CreateOpts{
			Name:            "test-image",
			ID:              "testID",
			ContainerFormat: "bare",
			DiskFormat:      "qcow2",
		},
	}

	ownerRef := &metav1.OwnerReference{
		APIVersion: openstackclusterstackrelease.APIVersion,
		Kind:       openstackclusterstackrelease.Kind,
		Name:       openstackclusterstackrelease.Name,
		UID:        openstackclusterstackrelease.UID,
	}

	secretName := "supersecret"
	secretNamespace := "test-namespace"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
		Type: corev1.SecretTypeOpaque,
	}
	err = client.Create(context.TODO(), secret)
	assert.NoError(t, err)

	r := &OpenStackClusterStackReleaseReconciler{
		Client: client,
	}

	assert.NoError(t, err)

	err = r.createOrUpdateOpenStackNodeImageRelease(context.TODO(), openstackclusterstackrelease, "test-osnir", openStackNodeImage, ownerRef)

	assert.NoError(t, err)
	osnir := &apiv1alpha1.OpenStackNodeImageRelease{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: "test-osnir", Namespace: "test-namespace"}, osnir)
	assert.NoError(t, err)
	assert.NotNil(t, osnir)

	expectedosnir := &apiv1alpha1.OpenStackNodeImageRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OpenStackNodeImageRelease",
			APIVersion: apiv1alpha1.GroupVersion.String(),
		},
		Spec: apiv1alpha1.OpenStackNodeImageReleaseSpec{
			IdentityRef: &capoapiv1alpha7.OpenStackIdentityReference{
				Kind: "Secret",
				Name: "supersecret",
			},
			Image: &apiv1alpha1.OpenStackNodeImage{
				URL: "test-url",
				CreateOpts: &apiv1alpha1.CreateOpts{
					Name:            "test-image",
					ID:              "testID",
					ContainerFormat: "bare",
					DiskFormat:      "qcow2",
				},
			},
		},
	}
	assert.Equal(t, expectedosnir.Spec, osnir.Spec)
	assert.Equal(t, expectedosnir.TypeMeta, osnir.TypeMeta)

	// Test cleanup
	err = client.Delete(context.TODO(), osnir)
	assert.NoError(t, err)
	err = client.Delete(context.TODO(), openstackclusterstackrelease)
	assert.NoError(t, err)
}

func TestUpdateOpenStackNodeImageRelease(t *testing.T) {
	scheme := runtime.NewScheme()
	err := apiv1alpha1.AddToScheme(scheme)
	assert.NoError(t, err)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	openstackclusterstackrelease := &apiv1alpha1.OpenStackClusterStackRelease{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "clusterstack.x-k8s.io/v1alpha1",
			Kind:       "OpenStackClusterStackRelease",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-release",
			Namespace: "test-namespace",
		},
		Spec: apiv1alpha1.OpenStackClusterStackReleaseSpec{
			IdentityRef: &capoapiv1alpha7.OpenStackIdentityReference{
				Kind: "Secret",
				Name: "supersecret",
			},
		},
	}
	assert.NoError(t, client.Create(context.TODO(), openstackclusterstackrelease))

	openStackNodeImage := &apiv1alpha1.OpenStackNodeImage{
		URL: "test-url",
		CreateOpts: &apiv1alpha1.CreateOpts{
			Name:            "test-image",
			ID:              "testID",
			ContainerFormat: "bare",
			DiskFormat:      "qcow2",
		},
	}

	ownerRef := &metav1.OwnerReference{
		APIVersion: openstackclusterstackrelease.APIVersion,
		Kind:       openstackclusterstackrelease.Kind,
		Name:       openstackclusterstackrelease.Name,
		UID:        "test-uid",
	}

	r := &OpenStackClusterStackReleaseReconciler{
		Client: client,
	}

	osnir := &apiv1alpha1.OpenStackNodeImageRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OpenStackNodeImageRelease",
			APIVersion: apiv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-update-osnir",
			Namespace: "test-namespace",
		},
		Spec: apiv1alpha1.OpenStackNodeImageReleaseSpec{
			IdentityRef: &capoapiv1alpha7.OpenStackIdentityReference{
				Kind: "Secret",
				Name: "supersecret",
			},
			Image: &apiv1alpha1.OpenStackNodeImage{
				URL: "test-url",
				CreateOpts: &apiv1alpha1.CreateOpts{
					Name:            "test-image",
					ID:              "testID",
					ContainerFormat: "bare",
					DiskFormat:      "qcow2",
				},
			},
		},
	}
	osnir.SetOwnerReferences([]metav1.OwnerReference{*ownerRef})
	assert.NoError(t, client.Create(context.TODO(), osnir))

	newOwnerRef := &metav1.OwnerReference{
		APIVersion: openstackclusterstackrelease.APIVersion,
		Kind:       openstackclusterstackrelease.Kind,
		Name:       openstackclusterstackrelease.Name,
		UID:        "test-new-uid",
	}

	err = client.Get(context.TODO(), types.NamespacedName{Name: "test-update-osnir", Namespace: "test-namespace"}, &apiv1alpha1.OpenStackNodeImageRelease{})
	assert.NoError(t, err)
	assert.Equal(t, ownerRef.UID, osnir.OwnerReferences[0].UID)

	err = r.createOrUpdateOpenStackNodeImageRelease(context.TODO(), openstackclusterstackrelease, "test-update-osnir", openStackNodeImage, newOwnerRef)
	assert.NoError(t, err)

	err = client.Get(context.TODO(), types.NamespacedName{Name: "test-update-osnir", Namespace: "test-namespace"}, osnir)
	assert.NoError(t, err)
	assert.NotNil(t, osnir)
	assert.Equal(t, *newOwnerRef, osnir.OwnerReferences[0])

	// Test cleanup
	err = client.Delete(context.TODO(), osnir)
	assert.NoError(t, err)
	err = client.Delete(context.TODO(), openstackclusterstackrelease)
	assert.NoError(t, err)
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
