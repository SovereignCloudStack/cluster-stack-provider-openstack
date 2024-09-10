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
	"fmt"
	"net/http"
	"testing"

	apiv1alpha1 "github.com/SovereignCloudStack/cluster-stack-provider-openstack/api/v1alpha1"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/imageimport"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/images"
	th "github.com/gophercloud/gophercloud/v2/testhelper"
	fakeclient "github.com/gophercloud/gophercloud/v2/testhelper/client"
	"github.com/gophercloud/utils/v2/openstack/clientconfig"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetCloudFromSecret(t *testing.T) {
	client := fake.NewClientBuilder().Build()

	secretName := "test-secret"
	secretNamespace := "test-namespace"
	cloudsYAML := `
clouds:
  openstack:
    auth:
      username: test_user
      password: test_password
      project_name: test_project
      project_id: test_project_id
      auth_url: test_auth_url
      domain_name: test_domain
    region_name: test_region
`

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
		Data: map[string][]byte{cloudsSecretKey: []byte(cloudsYAML)},
		Type: corev1.SecretTypeOpaque,
	}
	err := client.Create(context.TODO(), secret)
	assert.NoError(t, err)

	r := &OpenStackNodeImageReleaseReconciler{
		Client: client,
	}

	cloud, caCert, err := r.getCloudFromSecret(context.TODO(), secretNamespace, secretName)

	expectedCloud := clientconfig.Cloud{
		AuthInfo: &clientconfig.AuthInfo{
			Username:    "test_user",
			Password:    "test_password",
			ProjectName: "test_project",
			ProjectID:   "test_project_id",
			AuthURL:     "test_auth_url",
			DomainName:  "test_domain",
		},
		RegionName: "test_region",
	}
	assert.NoError(t, err)
	assert.Equal(t, expectedCloud, cloud)
	assert.Equal(t, []byte(nil), caCert)

	err = client.Delete(context.TODO(), secret)
	assert.NoError(t, err)
}

func TestGetCloudFromSecretWithCaCert(t *testing.T) {
	client := fake.NewClientBuilder().Build()

	secretName := "test-secret"
	secretNamespace := "test-namespace"
	cloudsYAML := `
clouds:
  openstack:
    auth:
      username: test_user
      password: test_password
      project_name: test_project
      project_id: test_project_id
      auth_url: test_auth_url
      domain_name: test_domain
    region_name: test_region
`
	expectedcaCert := []byte("test-ca-cert")

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
		Data: map[string][]byte{
			cloudsSecretKey: []byte(cloudsYAML),
			caSecretKey:     expectedcaCert,
		},
		Type: corev1.SecretTypeOpaque,
	}
	err := client.Create(context.TODO(), secret)
	assert.NoError(t, err)

	r := &OpenStackNodeImageReleaseReconciler{
		Client: client,
	}

	cloud, caCert, err := r.getCloudFromSecret(context.TODO(), secretNamespace, secretName)

	expectedCloud := clientconfig.Cloud{
		AuthInfo: &clientconfig.AuthInfo{
			Username:    "test_user",
			Password:    "test_password",
			ProjectName: "test_project",
			ProjectID:   "test_project_id",
			AuthURL:     "test_auth_url",
			DomainName:  "test_domain",
		},
		RegionName: "test_region",
	}
	assert.NoError(t, err)
	assert.Equal(t, expectedCloud, cloud)
	assert.Equal(t, caCert, expectedcaCert)

	err = client.Delete(context.TODO(), secret)
	assert.NoError(t, err)
}

func TestGetCloudFromSecretNotFound(t *testing.T) {
	client := fake.NewClientBuilder().Build()

	r := &OpenStackNodeImageReleaseReconciler{
		Client: client,
	}

	secretName := "nonexistent-secret"
	secretNamespace := "nonexistent-namespace"
	expectedError := "secrets \"nonexistent-secret\" not found"

	cloud, caCert, err := r.getCloudFromSecret(context.TODO(), secretNamespace, secretName)

	expectedErrorMessage := fmt.Sprintf("failed to get secret %s in namespace %s: %v", secretName, secretNamespace, expectedError)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
	assert.Equal(t, clientconfig.Cloud{}, cloud)
	assert.EqualError(t, err, expectedErrorMessage)
	assert.Equal(t, []byte(nil), caCert)
}

func TestGetCloudFromSecretMissingCloudsSecretKey(t *testing.T) {
	client := fake.NewClientBuilder().Build()

	r := &OpenStackNodeImageReleaseReconciler{
		Client: client,
	}

	secretName := "test-secret"
	secretNamespace := "test-namespace"

	// Create a secret with the bad cloudsSecretKey.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
		Data: map[string][]byte{"bad-clouds-secret-key": []byte("test-value")},
		Type: corev1.SecretTypeOpaque,
	}
	err := client.Create(context.TODO(), secret)
	assert.NoError(t, err)

	cloud, caCert, err := r.getCloudFromSecret(context.TODO(), secretNamespace, secretName)

	assert.Error(t, err)
	assert.EqualError(t, err, fmt.Sprintf("OpenStack credentials secret %s did not contain key %s", secretName, cloudsSecretKey))
	assert.Equal(t, clientconfig.Cloud{}, cloud)
	assert.Equal(t, []byte(nil), caCert)

	err = client.Delete(context.TODO(), secret)
	assert.NoError(t, err)
}

func TestGetCloudFromSecretCloudNotFound(t *testing.T) {
	client := fake.NewClientBuilder().Build()

	r := &OpenStackNodeImageReleaseReconciler{
		Client: client,
	}

	secretName := "test-secret"
	secretNamespace := "test-namespace"
	cloudName := "bad-cloud"
	cloudsYAML := `
clouds:
  openstack:
    auth:
      username: test_user
      password: test_password
      project_name: test_project
      project_id: test_project_id
      auth_url: test_auth_url
      domain_name: test_domain
    region_name: test_region
`

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
		Data: map[string][]byte{
			cloudsSecretKey:    []byte(cloudsYAML),
			cloudNameSecretKey: []byte(cloudName),
		},
		Type: corev1.SecretTypeOpaque,
	}
	err := client.Create(context.TODO(), secret)
	assert.NoError(t, err)

	cloud, caCert, err := r.getCloudFromSecret(context.TODO(), secretNamespace, secretName)

	assert.Error(t, err)
	assert.EqualError(t, err, fmt.Sprintf("failed to find cloud %s in %s", cloudName, cloudsSecretKey))
	assert.Equal(t, clientconfig.Cloud{}, cloud)
	assert.Equal(t, []byte(nil), caCert)

	err = client.Delete(context.TODO(), secret)
	assert.NoError(t, err)
}

func TestGetImageID(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	HandleImageListSuccessfully(t)

	imageFilter := &apiv1alpha1.CreateOpts{
		ID: "123",
	}

	imageID, err := getImageID(context.TODO(), fakeclient.ServiceClient(), imageFilter)

	assert.NoError(t, err)
	assert.Equal(t, "123", imageID)
}

func TestGetImageIDByNameAndTags(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	HandleImageListSuccessfully(t)

	imageFilter := &apiv1alpha1.CreateOpts{
		Name: "test_image",
		Tags: []string{"v1"},
	}

	imageID, err := getImageID(context.TODO(), fakeclient.ServiceClient(), imageFilter)

	assert.NoError(t, err)
	assert.Equal(t, "123", imageID)
}

// HandleImageListSuccessfully sets up a fake response for image list request.
func HandleImageListSuccessfully(t *testing.T) { //nolint: gocritic
	t.Helper() // Indicate that this is a test helper function
	th.Mux.HandleFunc("/images", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "GET")
		th.TestHeader(t, r, "X-Auth-Token", fakeclient.TokenID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{
			"images": [
				{"id": "123", "name": "test_image", "tags": ["v1"]}
			]
		}`)
	})
}

func TestGetImageIDWithTwoSameImageNames(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	HandleImageListWithTwoImagesSuccessfully(t)

	imageFilter := &apiv1alpha1.CreateOpts{
		Name: "test_image",
		Tags: []string{"v1"},
	}

	imageID, err := getImageID(context.TODO(), fakeclient.ServiceClient(), imageFilter)

	assert.Error(t, err) // Expecting an error due to multiple images with the same name
	assert.Equal(t, "", imageID)
	assert.Equal(t, err.Error(), "too many images were found with the given image name: test_image and tags: [v1]")
}

// HandleImageListWithTwoImagesSuccessfully sets up a fake response for image list request.
func HandleImageListWithTwoImagesSuccessfully(t *testing.T) { //nolint: gocritic
	t.Helper() // Indicate that this is a test helper function
	th.Mux.HandleFunc("/images", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "GET")
		th.TestHeader(t, r, "X-Auth-Token", fakeclient.TokenID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{
			"images": [
				{"id": "123", "name": "test_image", "tags": ["v1"]},
				{"id": "456", "name": "test_image", "tags": ["v1"]}
			]
		}`)
	})
}

func TestGetImageIDNoImageFound(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	HandleImageListWithNoImageFoundSuccessfully(t)

	imageFilter := &apiv1alpha1.CreateOpts{
		Name: "test_image",
		Tags: []string{"v1"},
	}

	imageID, err := getImageID(context.TODO(), fakeclient.ServiceClient(), imageFilter)

	assert.NoError(t, err)
	assert.Equal(t, "", imageID)
}

// HandleImageListWithNoImageFoundSuccessfully sets up a fake response for image list request.
func HandleImageListWithNoImageFoundSuccessfully(t *testing.T) { //nolint: gocritic
	t.Helper() // Indicate that this is a test helper function
	th.Mux.HandleFunc("/images", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "GET")
		th.TestHeader(t, r, "X-Auth-Token", fakeclient.TokenID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{
			"images": []
		}`)
	})
}

func TestGetImageIDWrongImageName(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	HandleImageListSuccessfully(t)

	imageFilter := &apiv1alpha1.CreateOpts{
		Name: "test_bad_image",
	}

	imageID, err := getImageID(context.TODO(), fakeclient.ServiceClient(), imageFilter)

	assert.NoError(t, err)
	assert.NotEqual(t, "231", imageID)
}

func TestGetImageIDNotFound(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	imageFilter := &apiv1alpha1.CreateOpts{
		Name: "test_image",
	}

	th.Mux.HandleFunc("/images", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, http.MethodGet)
		th.TestHeader(t, r, "X-Auth-Token", fakeclient.TokenID)

		// Respond with a 404 Not Found error.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error": {"message": "Image not found"}}`)
	})

	fakeClient := fakeclient.ServiceClient()

	imageID, err := getImageID(context.TODO(), fakeClient, imageFilter)

	assert.Error(t, err)
	assert.Equal(t, "", imageID)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to list images with name %s: Expected HTTP response code [200 204 300]", imageFilter.Name))
}

// HandleImageCreationSuccessfully test setup.
func HandleImageCreationSuccessfully(t *testing.T) { //nolint: gocritic
	t.Helper() // Indicate that this is a test helper function
	th.Mux.HandleFunc("/images", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "POST")
		th.TestHeader(t, r, "X-Auth-Token", fakeclient.TokenID)
		th.TestJSONRequest(t, r, `{
			"id": "test_id",
			"name": "test_image",
			"disk_format": "qcow2",
			"container_format": "bare"
		}`)

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{
			"status": "queued",
			"name": "test_image",
			"container_format": "bare",
			"disk_format": "qcow2",
			"visibility": "shared",
			"min_disk": 0,
			"id": "test_id"
		}`)
	})
}

func TestCreateImage(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	HandleImageCreationSuccessfully(t)

	createOpts := &apiv1alpha1.CreateOpts{
		ID:              "test_id",
		Name:            "test_image",
		DiskFormat:      "qcow2",
		ContainerFormat: "bare",
	}

	fakeClient := fakeclient.ServiceClient()

	createdImage, err := createImage(context.TODO(), fakeClient, createOpts)

	expectedImage := images.Image{
		ID:              "test_id",
		Name:            "test_image",
		Status:          "queued",
		ContainerFormat: "bare",
		DiskFormat:      "qcow2",
		Visibility:      "shared",
		Properties:      map[string]interface{}{},
	}

	assert.NoError(t, err)
	assert.NotNil(t, createdImage)
	assert.Equal(t, images.ImageVisibility("shared"), createdImage.Visibility)
	assert.Equal(t, &expectedImage, createdImage)
}

// HandleImageCreationWithError test setup.
func HandleImageCreationWithError(t *testing.T, errMsg string) {
	t.Helper() // Indicate that this is a test helper function
	th.Mux.HandleFunc("/images", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "POST")
		th.TestHeader(t, r, "X-Auth-Token", fakeclient.TokenID)
		th.TestJSONRequest(t, r, `{
			"id": "test_id",
			"name": "test_image",
			"disk_format": "qcow2",
			"container_format": "bare"
		}`)

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{
			"error": {
				"message": "%s"
			}
		}`, errMsg)
	})
}

func TestCreateImageFailed(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	HandleImageCreationWithError(t, "Internal Server Error")

	createOpts := &apiv1alpha1.CreateOpts{
		ID:              "test_id",
		Name:            "test_image",
		DiskFormat:      "qcow2",
		ContainerFormat: "bare",
	}

	fakeClient := fakeclient.ServiceClient()

	createdImage, err := createImage(context.TODO(), fakeClient, createOpts)

	assert.Error(t, err)
	assert.Nil(t, createdImage)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to create image with name %s: Expected HTTP response code [201]", createOpts.Name))
}

// HandleImageImportSuccessfully test setup.
func HandleImageImportSuccessfully(t *testing.T, imageID string) {
	t.Helper() // Indicate that this is a test helper function
	th.Mux.HandleFunc(fmt.Sprintf("/images/%s/import", imageID), func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "POST")
		th.TestHeader(t, r, "X-Auth-Token", fakeclient.TokenID)

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `{
			"status": "queued"
		}`)
	})
}

func TestImportImage(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	imageID := "test_image_id"
	HandleImageImportSuccessfully(t, imageID)

	createOpts := imageimport.CreateOpts{
		Name: imageimport.WebDownloadMethod,
	}

	fakeClient := fakeclient.ServiceClient()

	err := importImage(context.TODO(), fakeClient, imageID, createOpts)

	assert.NoError(t, err)
}

// HandleImageImportWithError test setup.
func HandleImageImportWithError(t *testing.T, imageID string, statusCode int, errMsg string) {
	t.Helper() // Indicate that this is a test helper function
	th.Mux.HandleFunc(fmt.Sprintf("/images/%s/import", imageID), func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "POST")
		th.TestHeader(t, r, "X-Auth-Token", fakeclient.TokenID)

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		fmt.Fprintf(w, `{
			"error": {
				"message": "%s"
			}
		}`, errMsg)
	})
}

func TestImportImageError(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	imageID := "test_image_id"
	HandleImageImportWithError(t, imageID, http.StatusInternalServerError, "Internal Server Error")

	createOpts := imageimport.CreateOpts{
		Name: "test_import_image",
	}

	fakeClient := fakeclient.ServiceClient()

	err := importImage(context.TODO(), fakeClient, imageID, createOpts)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to import image with ID %s: Expected HTTP response code [202]", imageID))
}
