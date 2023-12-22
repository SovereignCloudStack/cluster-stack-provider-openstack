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
	"errors"
	"fmt"
	"time"
	"sync"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/imageimport"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	apiv1alpha1 "github.com/sovereignCloudStack/cluster-stack-provider-openstack/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	type result struct {
		IsAvailable bool
		Err         error
	}

	ticker := time.Tick(interval)
	waiter := time.Tick(timeout)

	resultChannel := make(chan result)

	go func() {
		for {
			select {
			case _ = <-waiter:
				resultChannel <- result{IsAvailable: false, Err: errors.New("Timeout waiting for image to become active")}
				return
			case _ = <-ticker:
				image, err := images.Get(client, imageID).Extract()
				if err != nil {
					resultChannel <- result{IsAvailable: false, Err: err}
					return
				}

				if image.Status == "active" {
					resultChannel <- result{IsAvailable: true, Err: nil}
					return
				}
			}
		}
	}()

	resultStruct := <-resultChannel
	return resultStruct.IsAvailable, resultStruct.Err
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
		containerFormat string = openstacknodeimagerelease.Spec.ContainerFormat
		diskFormat      string = openstacknodeimagerelease.Spec.DiskFormat
	)

	// Create a channel to receive errors from the goroutine
	resultChan := make(chan error, 1)
	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Function to do import image concurrently, needs to be modified probably, still testing it
	downloadImage := func() {
		defer wg.Done() // Decrement the wait group counter when the goroutine completes
		cloud, err := getCloudFromSecret(ctx, r.Client, secretNamespace, secretName, cloudName)
		if err != nil {
			// Handle error
			resultChan <- err
			return
		}

		// Authenticate
		authOpts := gophercloud.AuthOptions{
			IdentityEndpoint: cloud.AuthInfo.AuthURL,
			Username:         cloud.AuthInfo.Username,
			Password:         cloud.AuthInfo.Password,
			DomainName:       cloud.AuthInfo.UserDomainName,
			TenantID:         cloud.AuthInfo.ProjectID,
		}

		provider, err := openstack.AuthenticatedClient(authOpts)
		if err != nil {
			// Handle error
			resultChan <- err
			return
		}

		// Create an Image service client
		imageClient, err := openstack.NewImageServiceV2(provider, gophercloud.EndpointOpts{
			Region: cloud.RegionName,
		})

		imageID, err := findImageByName(imageClient, imageName)
		if err != nil {
			resultChan <- fmt.Errorf("Error finding image: %w", err)
			return
		} else {
			if imageID == "" {
				visibility := images.ImageVisibilityShared

				createOptsImage := images.CreateOpts{
					Name:            imageName,
					ContainerFormat: containerFormat,
					DiskFormat:      diskFormat,
					Visibility:      &visibility,
				}

				image, err := createImage(imageClient, createOptsImage)
				if err != nil {
					// Handle error
					logger.Error(err, "Failed to find or create image")
					return
				}

				createOpts := imageimport.CreateOpts{
					Name: imageimport.WebDownloadMethod,
					URI:  imageURL,
				}
				imageID = image.ID

				// Handle error during image import
				err = importImage(imageClient, image.ID, createOpts)
				if err != nil {
					// Handle error
					logger.Error(err, "Failed to import image")
					return
				}
			}
		}
		// Check if image is active
		imageStatus, err = waitForImageActive(imageClient, imageID, 5*time.Second, 3*time.Minute)
		if err != nil {
			// Handle error
			logger.Error(err, "Failed to wait for image to become active")
			return
		} else if imageStatus {
			logger.Info("Image is active.")
		}
	}

	// Launch multiple goroutines to download images concurrently
	for i := 0; i < 1; i++ { // Adjust the number based on desired concurrency, needs to be test it :)
		wg.Add(1)
		go downloadImage()
	}

	// Wait for all goroutines to finish
	wg.Wait()

	return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackNodeImageReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.OpenStackNodeImageRelease{}).
		Complete(r)
}
