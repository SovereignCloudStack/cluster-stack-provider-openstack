/*
Copyright 2022 The Kubernetes Authors.

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

// Package helpers includes helper functions important for unit and integration testing.
package helpers

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	goruntime "runtime"

	githubclient "github.com/SovereignCloudStack/cluster-stack-operator/pkg/github/client"
	githubmocks "github.com/SovereignCloudStack/cluster-stack-operator/pkg/github/client/mocks"
	g "github.com/onsi/ginkgo/v2"
	cspov1alpha1 "github.com/sovereignCloudStack/cluster-stack-provider-openstack/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/log"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func init() {
	klog.InitFlags(nil)
	logger := klogr.New()

	// use klog as the internal logger for this envtest environment.
	log.SetLogger(logger)
	// additionally force all of the controllers to use the Ginkgo logger.
	ctrl.SetLogger(logger)
	// add logger for ginkgo
	klog.SetOutput(g.GinkgoWriter)
}

var (
	scheme = runtime.NewScheme()
	env    *envtest.Environment
	ctx    = context.Background()
)

const (
	// DefaultPodNamespace is default the namespace for the envtest resources.
	DefaultPodNamespace = "cspo-system"
)

func init() {
	// Calculate the scheme.
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(cspov1alpha1.AddToScheme(scheme))

	// Get the root of the current file to use in CRD paths.
	_, filename, _, _ := goruntime.Caller(0) //nolint:dogsled // external function
	root := path.Join(path.Dir(filename), "..", "..", "..")

	crdPaths := []string{
		filepath.Join(root, "config", "crd", "bases"),
	}

	// Create the test environment.
	env = &envtest.Environment{
		Scheme:                scheme,
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths:     crdPaths,
	}
}

type (
	// TestEnvironment encapsulates a Kubernetes local test environment.
	TestEnvironment struct {
		ctrl.Manager
		client.Client
		Config              *rest.Config
		cancel              context.CancelFunc
		GitHubClientFactory githubclient.Factory
		GitHubClient        *githubmocks.Client
	}
)

// NewTestEnvironment creates a new environment spinning up a local api-server.
func NewTestEnvironment() *TestEnvironment {
	config, err := env.Start()
	if err != nil {
		klog.Fatalf("unable to start env: %s", err)
	}

	// Build the controller manager.
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:  env.Scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	if err != nil {
		klog.Fatalf("unable to create manager: %s", err)
	}

	// create manager pod namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultPodNamespace,
		},
	}

	if err = mgr.GetClient().Create(ctx, ns); err != nil {
		klog.Fatalf("unable to create manager pod namespace: %s", err)
	}

	githubClient := &githubmocks.Client{}

	testEnv := &TestEnvironment{
		Manager:             mgr,
		Client:              mgr.GetClient(),
		Config:              mgr.GetConfig(),
		GitHubClientFactory: githubmocks.NewGitHubFactory(githubClient),
		GitHubClient:        githubClient,
	}

	return testEnv
}

// StartManager starts the manager and sets a cancel function into the testEnv object.
func (t *TestEnvironment) StartManager(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel
	if err := t.Manager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}
	return nil
}

// Stop stops the manager and cancels the context.
func (t *TestEnvironment) Stop() error {
	t.cancel()
	if err := env.Stop(); err != nil {
		return fmt.Errorf("failed to stop environment; %w", err)
	}
	return nil
}

// Cleanup deletes client objects.
func (t *TestEnvironment) Cleanup(ctx context.Context, objs ...client.Object) error {
	errs := make([]error, 0, len(objs))
	for _, o := range objs {
		err := t.Client.Delete(ctx, o)
		if apierrors.IsNotFound(err) {
			// If the object is not found, it must've been garbage collected
			// already. For example, if we delete namespace first and then
			// objects within it.
			continue
		}
		errs = append(errs, err)
	}
	return kerrors.NewAggregate(errs)
}

// CreateNamespace creates a namespace.
func (t *TestEnvironment) CreateNamespace(ctx context.Context, generateName string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", generateName),
			Labels: map[string]string{
				"testenv/original-name": generateName,
			},
		},
	}
	if err := t.Client.Create(ctx, ns); err != nil {
		return nil, fmt.Errorf("failed to create namespace: %w", err)
	}

	return ns, nil
}
