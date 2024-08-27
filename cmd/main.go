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

// Package main is the main package.
package main

// Import packages including all Kubernetes client auth plugins: k8s.io/client-go/plugin/pkg/client/auth.
import (
	"flag"
	"os"
	"time"

	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/assetsclient"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/assetsclient/fake"
	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/assetsclient/github"
	apiv1alpha1 "github.com/SovereignCloudStack/cluster-stack-provider-openstack/api/v1alpha1"
	"github.com/SovereignCloudStack/cluster-stack-provider-openstack/internal/controller"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeutils "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	k8sScheme = runtime.NewScheme()
	setupLog  = ctrl.Log.WithName("clusterstack-provider-openstack-setup")
)

func init() {
	runtimeutils.Must(scheme.AddToScheme(k8sScheme))
	runtimeutils.Must(apiv1alpha1.AddToScheme(k8sScheme))
}

var (
	releaseDir           string
	imageImportTimeout   int
	localMode            bool
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
)

func main() {
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(
		&enableLeaderElection,
		"leader-elect",
		false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.",
	)
	flag.StringVar(&releaseDir, "release-dir", "/tmp/downloads/", "Specify release directory for cluster-stack releases")
	flag.IntVar(&imageImportTimeout, "image-import-timeout", 0, "Maximum time in minutes that you allow cspo to import image. If image-import-timeout <= 0, cspo waits forever.")
	flag.BoolVar(&localMode, "local", false, "Enable local mode where no release assets will be downloaded from a remote repository. Useful for implementing cluster stacks.")

	opts := zap.Options{
		Development: true,
	}

	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	syncPeriod := 5 * time.Minute

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 k8sScheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "openstack.provider.clusterstack.x-k8s.io",
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize event recorder.
	record.InitFromRecorder(mgr.GetEventRecorderFor("cspo-controller"))

	var assetsClientFactory assetsclient.Factory
	if localMode {
		assetsClientFactory = fake.NewFactory()
	} else {
		assetsClientFactory = github.NewFactory()
	}

	if err = (&controller.OpenStackClusterStackReleaseReconciler{
		Client:              mgr.GetClient(),
		Scheme:              mgr.GetScheme(),
		ReleaseDirectory:    releaseDir,
		AssetsClientFactory: assetsClientFactory,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackClusterStackRelease")
		os.Exit(1)
	}
	if err = (&controller.OpenStackNodeImageReleaseReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		ImageImportTimeout: imageImportTimeout,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackNodeImageRelease")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
