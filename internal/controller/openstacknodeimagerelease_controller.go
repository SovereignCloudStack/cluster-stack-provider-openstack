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
	Scheme *runtime.Scheme
}

const (
	cloudsSecretKey = "clouds.yaml"
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
		openstacknodeimagerelease.Status.Ready = false

		imageCreateOpts := openstacknodeimagerelease.Spec.Image.CreateOpts
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

	// TODO: Add timeout logic - import start time could be taken from OpenStackImageNotImportedYetReason condition, or somehow better

	switch image.Status { //nolint:exhaustive
	case images.ImageStatusActive:
		logger.Info("OpenStackNodeImageRelease **ready** - image is **active**.", "name", openstacknodeimagerelease.Spec.Image.CreateOpts.Name, "ID", imageID)
		conditions.MarkTrue(openstacknodeimagerelease, apiv1alpha1.OpenStackImageReadyCondition)
		openstacknodeimagerelease.Status.Ready = true

		// requeue after 2 minutes to make sure the presence of the image
		return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Minute}, nil

	case images.ImageStatusImporting, images.ImageStatusSaving:

		logger.Info("OpenStackNodeImageRelease **not ready** yet - image is currently being imported by Glance.", "name", openstacknodeimagerelease.Spec.Image.CreateOpts.Name, "ID", imageID)
		conditions.MarkFalse(openstacknodeimagerelease, apiv1alpha1.OpenStackImageReadyCondition, apiv1alpha1.OpenStackImageNotImportedYetReason, clusterv1beta1.ConditionSeverityInfo, "image not imported yet")
		openstacknodeimagerelease.Status.Ready = false

		// wait for image - requeue after 30sec
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil

	case images.ImageStatusDeactivated:

		logger.Info("OpenStackNodeImageRelease **not ready** yet - image is deactivated.", "name", openstacknodeimagerelease.Spec.Image.CreateOpts.Name, "ID", imageID)
		conditions.MarkFalse(openstacknodeimagerelease, apiv1alpha1.OpenStackImageReadyCondition, apiv1alpha1.OpenStackImageIsDeactivatedReason, clusterv1beta1.ConditionSeverityWarning, "image is deactivated")
		openstacknodeimagerelease.Status.Ready = false

		// TODO: Should we make function to activate deactivated image or just tell user to activate it manually?
		// wait for image - requeue after 30sec
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil

	case images.ImageStatusKilled:

		logger.Info("OpenStackNodeImageRelease **error** - image is not readable.", "name", openstacknodeimagerelease.Spec.Image.CreateOpts.Name, "ID", imageID)
		conditions.MarkFalse(openstacknodeimagerelease, apiv1alpha1.OpenStackImageReadyCondition, apiv1alpha1.IssueWithOpenStackImageReason, clusterv1beta1.ConditionSeverityError, "image is not readable")
		openstacknodeimagerelease.Status.Ready = false

		// TODO: Image is broken and needs to be deleted?
		// wait for image - requeue after 30sec
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil

	case images.ImageStatusPendingDelete, images.ImageStatusDeleted:

		logger.Info("OpenStackNodeImageRelease **deleting** - image is being deleted.", "name", openstacknodeimagerelease.Spec.Image.CreateOpts.Name, "ID", imageID)
		conditions.MarkFalse(openstacknodeimagerelease, apiv1alpha1.OpenStackImageReadyCondition, apiv1alpha1.OpenStackImageIsDeletingReason, clusterv1beta1.ConditionSeverityInfo, "deleting image")
		openstacknodeimagerelease.Status.Ready = false

		// wait for image - requeue after 30sec
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil

	case images.ImageStatusQueued:

		logger.Info("OpenStackNodeImageRelease **not ready** - image is queued.", "name", openstacknodeimagerelease.Spec.Image.CreateOpts.Name, "ID", imageID)
		conditions.MarkFalse(openstacknodeimagerelease, apiv1alpha1.OpenStackImageReadyCondition, apiv1alpha1.OpenStackImageIsQueuedReason, clusterv1beta1.ConditionSeverityInfo, "image is queued")
		openstacknodeimagerelease.Status.Ready = false

		// wait for image - requeue after 30sec
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil

	// TODO: Choose what is default case for image.status
	default:
		logger.Info("OpenStackNodeImageRelease **handling for image status not defined yet** - requeue", "name", openstacknodeimagerelease.Spec.Image.CreateOpts.Name, "ID", imageID, "status", image.Status)

		// wait for image - requeue after 30sec
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
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
