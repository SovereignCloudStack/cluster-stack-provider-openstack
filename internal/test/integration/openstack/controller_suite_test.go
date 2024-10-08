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

package openstack

import (
	"testing"
	"time"

	"github.com/SovereignCloudStack/cluster-stack-provider-openstack/internal/controller"
	"github.com/SovereignCloudStack/cluster-stack-provider-openstack/internal/test/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	timeout  = time.Second * 40
	interval = 100 * time.Millisecond

	testClusterStackName = "openstack-integration-1-27-v1"
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
		AssetsClientFactory: testEnv.AssetsClientFactory,
		ReleaseDirectory:    "./../../../../test/releases",
	}).SetupWithManager(testEnv.Manager)).To(Succeed())

	Expect((&controller.OpenStackNodeImageReleaseReconciler{
		Client: testEnv.Manager.GetClient(),
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
