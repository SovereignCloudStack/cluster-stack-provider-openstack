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

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/imageimport"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"github.com/gophercloud/utils/openstack/clientconfig"
	apiv1alpha1 "github.com/sovereignCloudStack/cluster-stack-provider-openstack/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

// OpenStackNodeImageReleaseReconciler reconciles a OpenStackNodeImageRelease object.
type OpenStackNodeImageReleaseReconciler struct {
	client.Client
	Scheme                          *runtime.Scheme
	WaitForImageBecomeActiveMinutes int
}

const (
	cloudsSecretKey          = "clouds.yaml"
	waitForImageBecomeActive = 30 * time.Second
	reconcileImage           = 3 * time.Minute
)

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
func (r *OpenStackNodeImageReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	logger := log.FromContext(ctx)

	openstacknodeimagerelease := &apiv1alpha1.OpenStackNodeImageRelease{}
	err := r.Client.Get(ctx, req.NamespacedName, openstacknodeimagerelease)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get OpenStackNodeImageRelease %s/%s: %w", req.Namespace, req.Name, err)
	}

	patchHelper, err := patch.NewHelper(openstacknodeimagerelease, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to init patch helper for OpenStackNodeImageRelease: %w", err)
	}

	defer func() {
		conditions.SetSummary(openstacknodeimagerelease)

		if err := patchHelper.Patch(ctx, openstacknodeimagerelease); err != nil {
			reterr = fmt.Errorf("failed to patch OpenStackNodeImageRelease: %w", err)
		}
	}()

	// Get OpenStack cloud config from sercet
	cloud, err := r.getCloudFromSecret(ctx, openstacknodeimagerelease.Namespace, openstacknodeimagerelease.Spec.IdentityRef.Name, openstacknodeimagerelease.Spec.CloudName)
	if err != nil {
		conditions.MarkFalse(openstacknodeimagerelease,
			apiv1alpha1.CloudAvailableCondition,
			apiv1alpha1.CloudNotSetReason,
			clusterv1beta1.ConditionSeverityError,
			err.Error(),
		)
		record.Warnf(openstacknodeimagerelease, "CloudNotSetReason", err.Error())
		return ctrl.Result{}, fmt.Errorf("failed to get cloud from secret: %w", err)
	}

	conditions.MarkTrue(openstacknodeimagerelease, apiv1alpha1.CloudAvailableCondition)

	// Create an OpenStack provider client
	opts := &clientconfig.ClientOpts{AuthInfo: cloud.AuthInfo}
	providerClient, err := clientconfig.AuthenticatedClient(opts)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create a provider client: %w", err)
	}

	// Create an OpenStack image service client
	imageClient, err := openstack.NewImageServiceV2(providerClient, gophercloud.EndpointOpts{Region: cloud.RegionName})
	if err != nil {
		conditions.MarkFalse(openstacknodeimagerelease,
			apiv1alpha1.OpenStackImageServiceClientAvailableCondition,
			apiv1alpha1.OpenStackImageServiceClientNotSetReason,
			clusterv1beta1.ConditionSeverityError,
			err.Error(),
		)
		record.Warnf(openstacknodeimagerelease, "OpenStackImageServiceClientNotSet", err.Error())
		return ctrl.Result{}, fmt.Errorf("failed to create an image client: %w", err)
	}

	conditions.MarkTrue(openstacknodeimagerelease, apiv1alpha1.OpenStackImageServiceClientAvailableCondition)

	imageID, err := findImageByName(imageClient, openstacknodeimagerelease.Spec.Image.CreateOpts.Name)
	if err != nil {
		conditions.MarkFalse(openstacknodeimagerelease,
			apiv1alpha1.OpenStackImageReadyCondition,
			apiv1alpha1.IssueWithOpenStackImageReason,
			clusterv1beta1.ConditionSeverityError,
			err.Error(),
		)
		return ctrl.Result{}, fmt.Errorf("failed to find an image: %w", err)
	}

	if imageID == "" {
		conditions.MarkFalse(openstacknodeimagerelease, apiv1alpha1.OpenStackImageReadyCondition, apiv1alpha1.OpenStackImageNotCreatedYetReason, clusterv1beta1.ConditionSeverityInfo, "image is not created yet")
		conditions.MarkFalse(openstacknodeimagerelease, apiv1alpha1.OpenStackImageImportStartCondition, apiv1alpha1.OpenStackImageImportNotStartReason, clusterv1beta1.ConditionSeverityInfo, "image import not start yet")
		openstacknodeimagerelease.Status.Ready = false

		imageCreateOpts := (*images.CreateOpts)(openstacknodeimagerelease.Spec.Image.CreateOpts)
		imageCreated, err := createImage(imageClient, imageCreateOpts)
		if err != nil {
			conditions.MarkFalse(openstacknodeimagerelease,
				apiv1alpha1.OpenStackImageReadyCondition,
				apiv1alpha1.IssueWithOpenStackImageReason,
				clusterv1beta1.ConditionSeverityError,
				err.Error(),
			)
			return ctrl.Result{}, fmt.Errorf("failed to create an image: %w", err)
		}

		imageImportOpts := imageimport.CreateOpts{
			Name: imageimport.WebDownloadMethod,
			URI:  openstacknodeimagerelease.Spec.Image.URL,
		}
		err = importImage(imageClient, imageCreated.ID, imageImportOpts)
		if err != nil {
			conditions.MarkFalse(openstacknodeimagerelease,
				apiv1alpha1.OpenStackImageReadyCondition,
				apiv1alpha1.IssueWithOpenStackImageReason,
				clusterv1beta1.ConditionSeverityError,
				err.Error(),
			)
			return ctrl.Result{}, fmt.Errorf("failed to import an image: %w", err)
		}

		conditions.MarkTrue(openstacknodeimagerelease, apiv1alpha1.OpenStackImageImportStartCondition)
		// requeue to make sure that image ID can be find by image name
		return ctrl.Result{Requeue: true}, nil
	}

	// Check if image is active
	image, err := images.Get(imageClient, imageID).Extract()
	if err != nil {
		conditions.MarkFalse(openstacknodeimagerelease,
			apiv1alpha1.OpenStackImageReadyCondition,
			apiv1alpha1.IssueWithOpenStackImageReason,
			clusterv1beta1.ConditionSeverityError,
			err.Error(),
		)
		return ctrl.Result{}, fmt.Errorf("failed to get an image: %w", err)
	}

	// Check wait for image ACTIVE status duration
	if r.WaitForImageBecomeActiveMinutes > 0 && conditions.IsTrue(openstacknodeimagerelease, apiv1alpha1.OpenStackImageImportStartCondition) {
		// Calculate elapsed time since the OpenStackImageImportStartCondition is true
		startTime := conditions.GetLastTransitionTime(openstacknodeimagerelease, apiv1alpha1.OpenStackImageImportStartCondition)
		elapsedTime := time.Since(startTime.Time)

		waitForImageBecomeActiveTimeout := time.Duration(r.WaitForImageBecomeActiveMinutes) * time.Minute

		// Check if the image has been active after waitForImageBecomeActiveTimeout minutes
		if image.Status != images.ImageStatusActive && elapsedTime > waitForImageBecomeActiveTimeout {
			err = fmt.Errorf("timeout - wait for the image %s to transition to the ACTIVE status exceeds the timeout duration %d minutes", image.Name, r.WaitForImageBecomeActiveMinutes)
			logger.Error(err, "Timeout duration exceeded")
			conditions.MarkFalse(openstacknodeimagerelease,
				apiv1alpha1.OpenStackImageReadyCondition,
				apiv1alpha1.OpenStackImageImportTimeOutReason,
				clusterv1beta1.ConditionSeverityError,
				err.Error(),
			)
			// Image import timeout - nothing to do
			return ctrl.Result{}, nil
		}
	}

	// Manage image statuses according to the guidelines outlined in https://docs.openstack.org/glance/stein/user/statuses.html.
	switch image.Status {
	case images.ImageStatusActive:
		logger.Info("OpenStackNodeImageRelease **ready** - image is **ACTIVE**.", "name", openstacknodeimagerelease.Spec.Image.CreateOpts.Name, "ID", imageID)
		conditions.MarkTrue(openstacknodeimagerelease, apiv1alpha1.OpenStackImageReadyCondition)
		openstacknodeimagerelease.Status.Ready = true

	case images.ImageStatusDeactivated, images.ImageStatusKilled:
		// These statuses are unexpected. Hence we set a failure for them. See the explanation below:
		// `deactivated`: image is not allowed to use to any non-admin user
		// `killed`: an error occurred during the uploading of an image’s data, and that the image is not readable (via v1 API)
		err = fmt.Errorf("image status %s is unexpected", image.Status)
		conditions.MarkFalse(openstacknodeimagerelease,
			apiv1alpha1.OpenStackImageReadyCondition,
			apiv1alpha1.IssueWithOpenStackImageReason,
			clusterv1beta1.ConditionSeverityError,
			err.Error(),
		)
		openstacknodeimagerelease.Status.Ready = false
		return ctrl.Result{}, err

	case images.ImageStatusQueued, images.ImageStatusSaving, images.ImageStatusDeleted, images.ImageStatusPendingDelete, images.ImageStatusImporting:
		// The other statuses are expected. See the explanation below:
		// - `deleted`, `pending_delete`: The image has been deleted and will be removed soon, hence we can create and import the image in the next reconciliation loop.
		// - `importing`, `uploading`, `saving`, and `queued`: The image is in the process of being uploaded or imported via upload to Glance or the Glance image import API (performed by this reconciliation loop). Therefore, let's wait for it.
		logger.Info("OpenStackNodeImageRelease **not ready** yet - waiting for image to become ACTIVE", "name", openstacknodeimagerelease.Spec.Image.CreateOpts.Name, "ID", imageID, "status", image.Status)
		conditions.MarkFalse(openstacknodeimagerelease, apiv1alpha1.OpenStackImageReadyCondition, apiv1alpha1.OpenStackImageNotImportedYetReason, clusterv1beta1.ConditionSeverityInfo, "waiting for image to become ACTIVE")
		openstacknodeimagerelease.Status.Ready = false
		// Wait for image
		return ctrl.Result{RequeueAfter: waitForImageBecomeActive}, nil

	default:
		// An unknown state - set failure
		err = fmt.Errorf("image status %s is unknown", image.Status)
		conditions.MarkFalse(openstacknodeimagerelease,
			apiv1alpha1.OpenStackImageReadyCondition,
			apiv1alpha1.IssueWithOpenStackImageReason,
			clusterv1beta1.ConditionSeverityError,
			err.Error(),
		)
		openstacknodeimagerelease.Status.Ready = false
		return ctrl.Result{}, err
	}

	// Requeue to ensure the image's presence
	return ctrl.Result{Requeue: true, RequeueAfter: reconcileImage}, nil
}

func (r *OpenStackNodeImageReleaseReconciler) getCloudFromSecret(ctx context.Context, secretNamespace, secretName, cloudName string) (clientconfig.Cloud, error) {
	var clouds clientconfig.Clouds
	emptyCloud := clientconfig.Cloud{}

	if cloudName == "" {
		return emptyCloud, fmt.Errorf("secret name set to %s but no cloud was specified. Please set cloud_name in your machine spec", secretName)
	}

	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: secretNamespace,
		Name:      secretName,
	}, secret)
	if err != nil {
		return emptyCloud, fmt.Errorf("failed to get secret %s: %w", secretName, err)
	}
	content, ok := secret.Data[cloudsSecretKey]
	if !ok {
		return emptyCloud, fmt.Errorf("OpenStack credentials secret %s did not contain key %s", secretName, cloudsSecretKey)
	}
	if err = yaml.Unmarshal(content, &clouds); err != nil {
		return emptyCloud, fmt.Errorf("failed to unmarshal clouds credentials stored in secret %s: %w", secretName, err)
	}

	cloud, ok := clouds.Clouds[cloudName]
	if !ok {
		return emptyCloud, fmt.Errorf("failed to find cloud %s in %s", cloudName, cloudsSecretKey)
	}
	return cloud, nil
}

func findImageByName(imagesClient *gophercloud.ServiceClient, imageName string) (string, error) {
	listOpts := images.ListOpts{
		Name: imageName,
	}

	allPages, err := images.List(imagesClient, listOpts).AllPages()
	if err != nil {
		return "", fmt.Errorf("failed to list images with name %s: %w", imageName, err)
	}

	imageList, err := images.ExtractImages(allPages)
	if err != nil {
		return "", fmt.Errorf("failed to extract images with name %s: %w", imageName, err)
	}

	for i := range imageList {
		if imageList[i].Name == imageName {
			return imageList[i].ID, nil
		}
	}
	return "", nil
}

func createImage(imageClient *gophercloud.ServiceClient, createOpts *images.CreateOpts) (*images.Image, error) {
	image, err := images.Create(imageClient, createOpts).Extract()
	if err != nil {
		return nil, fmt.Errorf("failed to create image with name %s: %w", createOpts.Name, err)
	}

	return image, nil
}

func importImage(imageClient *gophercloud.ServiceClient, imageID string, createOpts imageimport.CreateOpts) error {
	err := imageimport.Create(imageClient, imageID, createOpts).ExtractErr()
	if err != nil {
		return fmt.Errorf("failed to import image with ID %s: %w", imageID, err)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackNodeImageReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.OpenStackNodeImageRelease{}).
		Complete(r)
}
