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
	"testing"

	"github.com/gophercloud/utils/openstack/clientconfig"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetCloudFromSecret(t *testing.T) {
	client := fake.NewClientBuilder().Build()

	secretName := "test-secret"
	secretNamespace := "test-namespace"
	cloudName := "openstack"
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
		Type: v1.SecretTypeOpaque,
	}
	err := client.Create(context.TODO(), secret)
	assert.NoError(t, err)

	r := &OpenStackNodeImageReleaseReconciler{
		Client: client,
	}

	cloud, err := r.getCloudFromSecret(context.TODO(), secretNamespace, secretName, cloudName)

	assert.NoError(t, err)
	assert.Equal(t, "test_user", cloud.AuthInfo.Username)
	assert.Equal(t, "test_password", cloud.AuthInfo.Password)
	assert.Equal(t, "test_project", cloud.AuthInfo.ProjectName)
	assert.Equal(t, "test_project_id", cloud.AuthInfo.ProjectID)
	assert.Equal(t, "test_auth_url", cloud.AuthInfo.AuthURL)
	assert.Equal(t, "test_domain", cloud.AuthInfo.DomainName)
	assert.Equal(t, "test_region", cloud.RegionName)

	// Cleanup the resources
	err = client.Delete(context.TODO(), secret)
	assert.NoError(t, err)
}

func TestGetCloudFromSecretNotFound(t *testing.T) {
	client := fake.NewClientBuilder().Build()

	r := &OpenStackNodeImageReleaseReconciler{
		Client: client,
	}

	cloud, err := r.getCloudFromSecret(context.TODO(), "nonexistent-namespace", "nonexistent-secret", "nonexistent-cloud")

	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
	assert.Equal(t, clientconfig.Cloud{}, cloud)
}

func TestGetCloudFromSecretMissingcloudsSecretKey(t *testing.T) {
	client := fake.NewClientBuilder().Build()

	r := &OpenStackNodeImageReleaseReconciler{
		Client: client,
	}

	// Create a secret with the bad cloudsSecretKey
	secretName := "test-secret"
	secretNamespace := "test-namespace"
	cloudName := "openstack"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
		Data: map[string][]byte{"bad-clouds-secret-key": []byte("test-value")},
		Type: v1.SecretTypeOpaque,
	}
	err := client.Create(context.TODO(), secret)
	assert.NoError(t, err)

	cloud, err := r.getCloudFromSecret(context.TODO(), secretNamespace, secretName, cloudName)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("OpenStack credentials secret %s did not contain key %s", secretName, cloudsSecretKey))
	assert.Equal(t, clientconfig.Cloud{}, cloud)

	// Cleanup the resources
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
		Data: map[string][]byte{cloudsSecretKey: []byte(cloudsYAML)},
		Type: v1.SecretTypeOpaque,
	}
	err := client.Create(context.TODO(), secret)
	assert.NoError(t, err)

	cloud, err := r.getCloudFromSecret(context.TODO(), secretNamespace, secretName, cloudName)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to find cloud %s in %s", cloudName, cloudsSecretKey))
	assert.Equal(t, clientconfig.Cloud{}, cloud)

	// Cleanup the resources
	err = client.Delete(context.TODO(), secret)
	assert.NoError(t, err)
}
