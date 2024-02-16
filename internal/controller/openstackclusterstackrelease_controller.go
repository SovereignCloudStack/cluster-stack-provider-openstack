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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	githubclient "github.com/SovereignCloudStack/cluster-stack-operator/pkg/github/client"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/release"
	apiv1alpha1 "github.com/sovereignCloudStack/cluster-stack-provider-openstack/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

// OpenStackClusterStackReleaseReconciler reconciles a OpenStackClusterStackRelease object.
type OpenStackClusterStackReleaseReconciler struct {
	client.Client
	Scheme                                         *runtime.Scheme
	GitHubClientFactory                            githubclient.Factory
	ReleaseDirectory                               string
	openStackClusterStackRelDownloadDirectoryMutex sync.Mutex
}

// NodeImages is the list of OpenStack images for the given cluster stack release.
type NodeImages struct {
	OpenStackNodeImages []*apiv1alpha1.OpenStackNodeImage `yaml:"openStackNodeImages"`
}

const (
	metadataFileName                             = "metadata.yaml"
	nodeImagesFileName                           = "node-images.yaml"
	waitForOpenStackNodeImageReleasesBecomeReady = 30 * time.Second
)

//+kubebuilder:rbac:groups=infrastructure.clusterstack.x-k8s.io,resources=openstackclusterstackreleases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.clusterstack.x-k8s.io,resources=openstackclusterstackreleases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.clusterstack.x-k8s.io,resources=openstackclusterstackreleases/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpenStackClusterStackRelease object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *OpenStackClusterStackReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	logger := log.FromContext(ctx)

	openstackclusterstackrelease := &apiv1alpha1.OpenStackClusterStackRelease{}
	err := r.Client.Get(ctx, req.NamespacedName, openstackclusterstackrelease)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get OpenStackClusterStackRelease %s/%s: %w", req.Namespace, req.Name, err)
	}

	patchHelper, err := patch.NewHelper(openstackclusterstackrelease, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to init patch helper for OpenStackClusterStackRelease: %w", err)
	}

	defer func() {
		conditions.SetSummary(openstackclusterstackrelease)

		if err := patchHelper.Patch(ctx, openstackclusterstackrelease); err != nil {
			reterr = fmt.Errorf("failed to patch OpenStackClusterStackRelease: %w", err)
		}
	}()

	// name of OpenStackClusterStackRelease object is same as the release tag
	releaseTag := openstackclusterstackrelease.Name

	releaseAssets, download, err := release.New(releaseTag, r.ReleaseDirectory)
	if err != nil {
		conditions.MarkFalse(openstackclusterstackrelease,
			apiv1alpha1.ClusterStackReleaseAssetsReadyCondition,
			apiv1alpha1.IssueWithReleaseAssetsReason,
			clusterv1beta1.ConditionSeverityError,
			err.Error(),
		)
		record.Warnf(openstackclusterstackrelease, "IssueWithReleaseAssets", err.Error())
		logger.Error(err, "failed to create release")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	if download {
		conditions.MarkFalse(openstackclusterstackrelease, apiv1alpha1.ClusterStackReleaseAssetsReadyCondition, apiv1alpha1.ReleaseAssetsNotDownloadedYetReason, clusterv1beta1.ConditionSeverityInfo, "assets not downloaded yet")

		// this is the point where we download the release from github
		// acquire lock so that only one reconcile loop can download the release
		r.openStackClusterStackRelDownloadDirectoryMutex.Lock()

		gc, err := r.GitHubClientFactory.NewClient(ctx)
		if err != nil {
			conditions.MarkFalse(openstackclusterstackrelease,
				apiv1alpha1.GitAPIAvailableCondition,
				apiv1alpha1.GitTokenOrEnvVariableNotSetReason,
				clusterv1beta1.ConditionSeverityError,
				err.Error(),
			)
			record.Warnf(openstackclusterstackrelease, "GitTokenOrEnvVariableNotSet", err.Error())
			logger.Error(err, "failed to create Github client")
			return ctrl.Result{}, nil
		}

		conditions.MarkTrue(openstackclusterstackrelease, apiv1alpha1.GitAPIAvailableCondition)

		if err := downloadReleaseAssets(ctx, releaseTag, releaseAssets.LocalDownloadPath, gc); err != nil {
			logger.Error(err, "failed to download release assets")
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}

		r.openStackClusterStackRelDownloadDirectoryMutex.Unlock()

		record.Eventf(openstackclusterstackrelease, "ClusterStackReleaseAssetsReady", "successfully downloaded ClusterStackReleaseAssets %q", releaseTag)
		// requeue to make sure release assets can be accessed
		return ctrl.Result{Requeue: true}, nil
	}

	conditions.MarkTrue(openstackclusterstackrelease, apiv1alpha1.ClusterStackReleaseAssetsReadyCondition)

	nodeImages, err := getNodeImagesFromLocal(releaseAssets.LocalDownloadPath)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get node images from local: %w", err)
	}
	ownerRef := generateOwnerReference(openstackclusterstackrelease)

	for _, openStackNodeImage := range nodeImages.OpenStackNodeImages {
		// OpenStackNodeImageRelease is composed as follows:
		// <release tag without version>-<image name>-<node-image-version>
		// e.g.: `openstack-ferrol-1-27-ubuntu-capi-image-v1.27.8-v2`
		// This ensures that multiple versions of one ClusterStack could share images
		nodeImageVersion := releaseAssets.Meta.Versions.Components.NodeImage
		nameWithoutVersion, err := cutOpenStackClusterStackReleaseVersionFromReleaseTag(openstackclusterstackrelease.Name)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to cut release tag: %w", err)
		}

		osnirName := fmt.Sprintf("%s-%s-%s", nameWithoutVersion, openStackNodeImage.CreateOpts.Name, nodeImageVersion)

		if err := r.createOrUpdateOpenStackNodeImageRelease(ctx, openstackclusterstackrelease, osnirName, openStackNodeImage, ownerRef); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create or update OpenStackNodeImageRelease %s/%s: %w", openstackclusterstackrelease.Namespace, osnirName, err)
		}
	}

	ownedOpenStackNodeImageReleases, err := r.getOwnedOpenStackNodeImageReleases(ctx, openstackclusterstackrelease)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get owned OpenStackNodeImageReleases: %w", err)
	}

	if len(ownedOpenStackNodeImageReleases) == 0 {
		logger.Info("OpenStackClusterStackRelease **not ready** yet - waiting for OpenStackNodeImageReleases to be created")
		conditions.MarkFalse(openstackclusterstackrelease,
			apiv1alpha1.OpenStackNodeImageReleasesReadyCondition,
			apiv1alpha1.ProcessOngoingReason, clusterv1beta1.ConditionSeverityInfo,
			"OpenStackNodeImageReleases not ready yet",
		)
		openstackclusterstackrelease.Status.Ready = false
		return ctrl.Result{RequeueAfter: waitForOpenStackNodeImageReleasesBecomeReady}, nil
	}
	for _, openStackNodeImageRelease := range ownedOpenStackNodeImageReleases {
		// TODO: Handle case when `import-timeout > 0`. Then the oscsr_controller should stop the reconciliation
		if openStackNodeImageRelease.Status.Ready {
			continue
		}
		logger.Info("OpenStackClusterStackRelease **not ready** yet - waiting for OpenStackNodeImageRelease to be ready", "name:", openStackNodeImageRelease.ObjectMeta.Name, "ready:", openStackNodeImageRelease.Status.Ready)
		conditions.MarkFalse(openstackclusterstackrelease,
			apiv1alpha1.OpenStackNodeImageReleasesReadyCondition,
			apiv1alpha1.ProcessOngoingReason, clusterv1beta1.ConditionSeverityInfo,
			"OpenStackNodeImageReleases not ready yet",
		)
		openstackclusterstackrelease.Status.Ready = false
		return ctrl.Result{RequeueAfter: waitForOpenStackNodeImageReleasesBecomeReady}, nil
	}

	logger.Info("OpenStackClusterStackRelease **ready**")
	conditions.MarkTrue(openstackclusterstackrelease, apiv1alpha1.OpenStackNodeImageReleasesReadyCondition)
	record.Eventf(openstackclusterstackrelease, "OpenStackNodeImageReleasesReady", "OpenStackNodeImageRelease objects are ready")
	openstackclusterstackrelease.Status.Ready = true

	return ctrl.Result{}, nil
}

func (r *OpenStackClusterStackReleaseReconciler) createOrUpdateOpenStackNodeImageRelease(ctx context.Context, openstackclusterstackrelease *apiv1alpha1.OpenStackClusterStackRelease, osnirName string, openStackNodeImage *apiv1alpha1.OpenStackNodeImage, ownerRef *metav1.OwnerReference) error {
	openStackNodeImageRelease := &apiv1alpha1.OpenStackNodeImageRelease{}

	err := r.Get(ctx, types.NamespacedName{Name: osnirName, Namespace: openstackclusterstackrelease.Namespace}, openStackNodeImageRelease)

	// Update owner references if the object exists
	if err == nil {
		// Ensure owner reference
		openStackNodeImageRelease.SetOwnerReferences(util.EnsureOwnerRef(openStackNodeImageRelease.GetOwnerReferences(), *ownerRef))

		if err := r.Update(ctx, openStackNodeImageRelease); err != nil {
			record.Warnf(openStackNodeImageRelease, "FailedUpdateOpenStackNodeImageRelease", err.Error())
			return fmt.Errorf("failed to update OpenStackNodeImageRelease: %w", err)
		}

		return nil
	}

	// Unexpected error
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get OpenStackNodeImageRelease: %w", err)
	}

	// Object not found - create it
	openStackNodeImageRelease.Name = osnirName
	openStackNodeImageRelease.Namespace = openstackclusterstackrelease.Namespace
	openStackNodeImageRelease.TypeMeta = metav1.TypeMeta{
		Kind:       "OpenStackNodeImageRelease",
		APIVersion: apiv1alpha1.GroupVersion.String(),
	}
	openStackNodeImageRelease.SetOwnerReferences([]metav1.OwnerReference{*ownerRef})
	openStackNodeImageRelease.Spec.Image = openStackNodeImage
	openStackNodeImageRelease.Spec.CloudName = openstackclusterstackrelease.Spec.CloudName
	openStackNodeImageRelease.Spec.IdentityRef = openstackclusterstackrelease.Spec.IdentityRef

	if err := r.Create(ctx, openStackNodeImageRelease); err != nil {
		record.Warnf(openStackNodeImageRelease, "FailedCreateOpenStackNodeImageRelease", err.Error())
		return fmt.Errorf("failed to create OpenStackNodeImageRelease: %w", err)
	}

	record.Eventf(openstackclusterstackrelease, "OpenStackNodeImageReleaseCreated", "successfully created OpenStackNodeImageRelease object %q", osnirName)
	return nil
}

func (r *OpenStackClusterStackReleaseReconciler) getOwnedOpenStackNodeImageReleases(ctx context.Context, openstackclusterstackrelease *apiv1alpha1.OpenStackClusterStackRelease) ([]*apiv1alpha1.OpenStackNodeImageRelease, error) {
	osnirList := &apiv1alpha1.OpenStackNodeImageReleaseList{}

	if err := r.List(ctx, osnirList, client.InNamespace(openstackclusterstackrelease.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list OpenStackNodeImageReleases: %w", err)
	}

	ownedOpenStackNodeImageReleases := make([]*apiv1alpha1.OpenStackNodeImageRelease, 0, len(osnirList.Items))

	for i := range osnirList.Items {
		osnir := osnirList.Items[i]
		for i := range osnir.GetOwnerReferences() {
			ownerRef := osnir.GetOwnerReferences()[i]
			if matchOwnerReference(&ownerRef, openstackclusterstackrelease) {
				ownedOpenStackNodeImageReleases = append(ownedOpenStackNodeImageReleases, &osnir)
				break
			}
		}
	}
	return ownedOpenStackNodeImageReleases, nil
}

func downloadReleaseAssets(ctx context.Context, releaseTag, downloadPath string, gc githubclient.Client) error {
	repoRelease, resp, err := gc.GetReleaseByTag(ctx, releaseTag)
	if err != nil {
		return fmt.Errorf("failed to fetch release tag %q: %w", releaseTag, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch release tag %s with status code %d", releaseTag, resp.StatusCode)
	}

	assetlist := []string{metadataFileName, nodeImagesFileName}

	if err := gc.DownloadReleaseAssets(ctx, repoRelease, downloadPath, assetlist); err != nil {
		// if download failed for some reason, delete the release directory so that it can be retried in the next reconciliation
		if err := os.RemoveAll(downloadPath); err != nil {
			return fmt.Errorf("failed to remove release: %w", err)
		}
		return fmt.Errorf("failed to download release assets: %w", err)
	}

	return nil
}

func generateOwnerReference(openstackClusterStackRelease *apiv1alpha1.OpenStackClusterStackRelease) *metav1.OwnerReference {
	return &metav1.OwnerReference{
		APIVersion: openstackClusterStackRelease.APIVersion,
		Kind:       openstackClusterStackRelease.Kind,
		Name:       openstackClusterStackRelease.Name,
		UID:        openstackClusterStackRelease.UID,
	}
}

func matchOwnerReference(a *metav1.OwnerReference, openstackclusterstackrelease *apiv1alpha1.OpenStackClusterStackRelease) bool {
	aGV, err := schema.ParseGroupVersion(a.APIVersion)
	if err != nil {
		return false
	}

	return aGV.Group == openstackclusterstackrelease.GroupVersionKind().Group && a.Kind == openstackclusterstackrelease.Kind && a.Name == openstackclusterstackrelease.Name
}

func getNodeImagesFromLocal(localDownloadPath string) (*NodeImages, error) {
	// Read the node-images.yaml file from the release
	nodeImagePath := filepath.Join(localDownloadPath, nodeImagesFileName)
	f, err := os.ReadFile(filepath.Clean(nodeImagePath))
	if err != nil {
		return nil, fmt.Errorf("failed to read node-images file %s: %w", nodeImagePath, err)
	}
	nodeImages := NodeImages{}
	// if unmarshal fails, it indicates incomplete node-images file.
	// But we don't want to enforce download again.
	if err = yaml.Unmarshal(f, &nodeImages); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node-images: %w", err)
	}
	return &nodeImages, nil
}

// cutOpenStackClusterStackReleaseVersionFromReleaseTag returns a release tag without version,
// e.g.  from `openstack-ferrol-1-27-v2` returns `openstack-ferrol-1-27`.
func cutOpenStackClusterStackReleaseVersionFromReleaseTag(releaseTag string) (string, error) {
	v := strings.Split(releaseTag, "-")
	if len(v) != 5 && len(v) != 6 {
		return "", fmt.Errorf("invalid release tag %s", releaseTag)
	}
	return fmt.Sprintf("%s-%s-%s-%s", v[0], v[1], v[2], v[3]), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackClusterStackReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.OpenStackClusterStackRelease{}).
		Complete(r)
}
