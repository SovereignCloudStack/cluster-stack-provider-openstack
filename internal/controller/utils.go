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

	"sigs.k8s.io/controller-runtime/pkg/client"
	"github.com/gophercloud/utils/openstack/clientconfig"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
	"k8s.io/apimachinery/pkg/types"

)

func getCloudFromSecret(ctx context.Context, ctrlClient client.Client, secretNamespace string, secretName string, cloudName string) (clientconfig.Cloud, error) {
	const (
		cloudsSecretKey = "clouds.yaml"
	)
	
	emptyCloud := clientconfig.Cloud{}

	if cloudName == "" {
		return emptyCloud, fmt.Errorf("secret name set to %v but no cloud was specified. Please set cloud_name in your machine spec", secretName)
	}
	secret := &corev1.Secret{}
	err := ctrlClient.Get(ctx, types.NamespacedName{
		Namespace: secretNamespace,
		Name:      secretName,
	}, secret)
	if err != nil {
		return emptyCloud, err
	}
	content, ok := secret.Data[cloudsSecretKey]
	if !ok {
		return emptyCloud, fmt.Errorf("OpenStack credentials secret %v did not contain key %v",
			secretName, cloudsSecretKey)
	}
	var clouds clientconfig.Clouds
	if err = yaml.Unmarshal(content, &clouds); err != nil {
		return emptyCloud, fmt.Errorf("failed to unmarshal clouds credentials stored in secret %v: %v", secretName, err)
	}

	return clouds.Clouds[cloudName], nil
}
