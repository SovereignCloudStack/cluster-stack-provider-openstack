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

package oci

import (
	"testing"
	"time"

	ociclient "github.com/SovereignCloudStack/cluster-stack-operator/pkg/assetsclient/oci"
	"github.com/SovereignCloudStack/cluster-stack-provider-openstack/internal/controller"
	"github.com/SovereignCloudStack/cluster-stack-provider-openstack/internal/test/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	timeout    = time.Second * 10
	interval   = 1000 * time.Millisecond
	releaseDir = "/tmp/downloads"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var (
	ctx     = ctrl.SetupSignalHandler()
	testEnv *helpers.TestEnvironment
)

var _ = BeforeSuite(func() {
	testEnv = helpers.NewTestEnvironment()
	Expect((&controller.OpenStackClusterStackReleaseReconciler{
		Client:              testEnv.Manager.GetClient(),
		AssetsClientFactory: ociclient.NewFactory(),
		ReleaseDirectory:    releaseDir,
	}).SetupWithManager(testEnv.Manager)).To(Succeed())

	go func() {
		defer GinkgoRecover()
		Expect(testEnv.StartManager(ctx)).To(Succeed())
	}()
	<-testEnv.Manager.Elected()
})

var _ = AfterSuite(func() {
	Expect(testEnv.Stop()).To(Succeed())
})
