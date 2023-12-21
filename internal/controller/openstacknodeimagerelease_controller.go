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

// Package controller implements the controller logic.
package controller

import (
	"context"
	"fmt"
	"time"

	apiv1alpha1 "github.com/sovereignCloudStack/cluster-stack-provider-openstack/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/imageimport"
)

// OpenStackNodeImageReleaseReconciler reconciles a OpenStackNodeImageRelease object.
type OpenStackNodeImageReleaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func findImageByName(imagesClient *gophercloud.ServiceClient, imageName string) (string, error) {
	var imageID string

	listOpts := images.ListOpts{
		Name: imageName,
	}

	allPages, err := images.List(imagesClient, listOpts).AllPages()
	imageList, err := images.ExtractImages(allPages)
	for _, image := range imageList {
		if image.Name == imageName {
			imageID = image.ID
			break
		}
	}

	if err != nil {
		return "", err
	}

	return imageID, nil
}

func waitForImageActive(client *gophercloud.ServiceClient, imageID string, interval time.Duration, timeout time.Duration) (bool, error) {
	startTime := time.Now()

	for {
		image, err := images.Get(client, imageID).Extract()
		if err != nil {
			return false, err
		}

		if image.Status == "active" {
			return true, nil
		}

		if time.Since(startTime) > timeout {
			return false, fmt.Errorf("Timeout waiting for image to become active")
		}

		time.Sleep(interval)
	}
}

func createImage(imageClient *gophercloud.ServiceClient, createOpts images.CreateOpts) (*images.Image, error) {
	image, err := images.Create(imageClient, createOpts).Extract()
	if err != nil {
		return nil, err
	}

	return image, nil
}

func importImage(imageClient *gophercloud.ServiceClient, imageID string, createOpts imageimport.CreateOpts) error {
	err := imageimport.Create(imageClient, imageID, createOpts).ExtractErr()
	if err != nil {
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=infrastructure.clusterstack.x-k8s.io,resources=openstacknodeimagereleases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.clusterstack.x-k8s.io,resources=openstacknodeimagereleases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.clusterstack.x-k8s.io,resources=openstacknodeimagereleases/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpenStackNodeImageRelease object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *OpenStackNodeImageReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	openstacknodeimagerelease := apiv1alpha1.OpenStackNodeImageRelease{}
	err := r.Client.Get(ctx, req.NamespacedName, &openstacknodeimagerelease)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("Failed to get OpenStackNodeImageRelease %s/%s: %w", req.Namespace, req.Name, err)
	}

	var (
		imageStatus     bool   = false
		secretName      string = openstacknodeimagerelease.Spec.IdentityRef.Name
		secretNamespace string = "default"
		cloudName       string = openstacknodeimagerelease.Spec.CloudName
		imageName       string = openstacknodeimagerelease.Spec.Name
		imageURL        string = openstacknodeimagerelease.Spec.URL
	)

	cloud, err := getCloudFromSecret(ctx, r.Client, secretNamespace, secretName, cloudName)
	if err != nil {
		return ctrl.Result{}, err
	}
	var (
		username       string = cloud.AuthInfo.Username
		authURL        string = cloud.AuthInfo.AuthURL
		projectID      string = cloud.AuthInfo.ProjectID
		userDomainName string = cloud.AuthInfo.UserDomainName
		password       string = cloud.AuthInfo.Password
	)

	// Authenticate
	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: authURL,
		Username:         username,
		Password:         password,
		DomainName:       userDomainName,
		TenantID:         projectID,
	}

	provider, err := openstack.AuthenticatedClient(authOpts)
	if err != nil {
		fmt.Errorf("Error authenticating with OpenStack: %v", err)
	}

	// Create an Image service client
	imageClient, err := openstack.NewImageServiceV2(provider, gophercloud.EndpointOpts{
		Region: cloud.RegionName,
	})

	imageID, err := findImageByName(imageClient, imageName)
	if err != nil {
		fmt.Errorf("Error finding image: %w", err)
	} else {
		if imageID == "" {
			visibility := images.ImageVisibilityShared
	
			createOptsImage := images.CreateOpts{
				Name:            imageName,
				ContainerFormat: "bare",				
				DiskFormat:      "iso",
				Visibility: 	 &visibility,
			}
	
			image, err := createImage(imageClient, createOptsImage)
			if err != nil {
				return ctrl.Result{}, err
			}
	
			createOpts := imageimport.CreateOpts{
				Name: imageimport.WebDownloadMethod,
				URI:  imageURL,
			}
			imageID = image.ID
	
			// Handle error during image import
			err = importImage(imageClient, image.ID, createOpts)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Check if image is active
	imageStatus, err = waitForImageActive(imageClient, imageID, 5*time.Second, 10*time.Minute)
	if err != nil {
		fmt.Errorf("Error waiting for image to become active:", err)
	}
	if imageStatus {
		logger.Info("Image is active.")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackNodeImageReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.OpenStackNodeImageRelease{}).
		Complete(r)
}
