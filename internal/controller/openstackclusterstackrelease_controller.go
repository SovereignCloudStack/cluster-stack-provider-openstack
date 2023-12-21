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
	"sync"
	"time"

	githubclient "github.com/SovereignCloudStack/cluster-stack-operator/pkg/github/client"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/release"
	apiv1alpha1 "github.com/sovereignCloudStack/cluster-stack-provider-openstack/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// OpenStackClusterStackReleaseReconciler reconciles a OpenStackClusterStackRelease object.
type OpenStackClusterStackReleaseReconciler struct {
	client.Client
	Scheme                                         *runtime.Scheme
	GitHubClientFactory                            githubclient.Factory
	ReleaseDirectory                               string
	openStackClusterStackRelDownloadDirectoryMutex sync.Mutex
}

//+kubebuilder:rbac:groups=infrastructure.clusterstack.x-k8s.io,resources=openstackclusterstackreleases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.clusterstack.x-k8s.io,resources=openstackclusterstackreleases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.clusterstack.x-k8s.io,resources=openstackclusterstackreleases/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpenStackClusterStackRelease object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *OpenStackClusterStackReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	openstackclusterstackrelease := &apiv1alpha1.OpenStackClusterStackRelease{}
	err := r.Client.Get(ctx, req.NamespacedName, openstackclusterstackrelease)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get OpenStackClusterStackRelease %s/%s: %w", req.Namespace, req.Name, err)
	}

	gc, err := r.GitHubClientFactory.NewClient(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create Github client: %w", err)
	}

	// name of OpenStackClusterStackRelease object is same as the release tag
	releaseTag := openstackclusterstackrelease.Name

	releaseAssets, download, err := release.New(releaseTag, r.ReleaseDirectory)
	if err != nil {
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, fmt.Errorf("failed to create release: %w", err)
	}

	if download {
		// this is the point where we download the release from github
		// acquire lock so that only one reconcile loop can download the release
		r.openStackClusterStackRelDownloadDirectoryMutex.Lock()

		if err := downloadReleaseAssets(ctx, releaseTag, releaseAssets.LocalDownloadPath, gc); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to download release assets: %w", err)
		}

		r.openStackClusterStackRelDownloadDirectoryMutex.Unlock()

		// requeue to make sure release assets can be accessed
		return ctrl.Result{Requeue: true}, nil
	}

	logger.Info("OpenStackClusterStackRelease status", "ready", openstackclusterstackrelease.Status.Ready)
	openstackclusterstackrelease.Status.Ready = true
	err = r.Status().Update(ctx, openstackclusterstackrelease)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update OpenStackClusterStackRelease status")
	}
	logger.Info("OpenStackClusterStackRelease ready")

	return ctrl.Result{}, nil
}

func downloadReleaseAssets(ctx context.Context, releaseTag, downloadPath string, gc githubclient.Client) error {
	repoRelease, resp, err := gc.GetReleaseByTag(ctx, releaseTag)
	if err != nil {
		return fmt.Errorf("failed to fetch release tag %q: %w", releaseTag, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch release tag %s with status code %d", releaseTag, resp.StatusCode)
	}

	assetlist := []string{"metadata.yaml", "node-images.yaml"}

	if err := gc.DownloadReleaseAssets(ctx, repoRelease, downloadPath, assetlist); err != nil {
		// if download failed for some reason, delete the release directory so that it can be retried in the next reconciliation
		if err := os.RemoveAll(downloadPath); err != nil {
			return fmt.Errorf("failed to remove release: %w", err)
		}
		return fmt.Errorf("failed to download release assets: %w", err)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackClusterStackReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.OpenStackClusterStackRelease{}).
		Complete(r)
}
