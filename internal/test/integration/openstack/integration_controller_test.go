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
	"encoding/base64"
	"os"

	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cspov1alpha1 "github.com/sovereignCloudStack/cluster-stack-provider-openstack/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apiv1alpha7 "sigs.k8s.io/cluster-api-provider-openstack/api/v1alpha7"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var _ = Describe("Test", func() {
	Context("test", func() {
		var (
			openStackClusterStackRelease *cspov1alpha1.OpenStackClusterStackRelease
			testNs                       *corev1.Namespace
			openStackNodeImageRelease    cspov1alpha1.OpenStackNodeImageRelease
		)

		BeforeEach(func() {
			var err error
			testNs, err = testEnv.CreateNamespace(ctx, "osnir-integration")
			Expect(err).NotTo(HaveOccurred())

			cloudsYAMLBase64 := os.Getenv("ENCODED_CLOUDS_YAML")
			cloudsYAMLData, err := base64.StdEncoding.DecodeString(cloudsYAMLBase64)
			Expect(err).NotTo(HaveOccurred())

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "supersecret",
					Namespace: testNs.Name,
				},
				Data: map[string][]byte{
					"clouds.yaml": cloudsYAMLData,
				},
			}
			Expect(testEnv.Create(ctx, secret)).To(Succeed())

			openStackClusterStackRelease = &cspov1alpha1.OpenStackClusterStackRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       "OpenStackClusterStackRelease",
					APIVersion: "infrastructure.clusterstack.x-k8s.io",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      testClusterStackName,
					Namespace: testNs.Name,
				},
				Spec: cspov1alpha1.OpenStackClusterStackReleaseSpec{
					CloudName: "openstack",
					IdentityRef: &apiv1alpha7.OpenStackIdentityReference{
						Kind: "Secret",
						Name: "supersecret",
					},
				},
				Status: cspov1alpha1.OpenStackClusterStackReleaseStatus{
					Ready:      false,
					Conditions: clusterv1beta1.Conditions{},
				},
			}
			Expect(testEnv.Create(ctx, openStackClusterStackRelease)).To(Succeed())
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, testNs, openStackClusterStackRelease)).To(Succeed())
		})

		It("creates the OpenStackNodeImageRelease object", func() {
			openstackNodeImageReleaseName := types.NamespacedName{Name: "openstack-integration-1-27-ubuntu-test-image-v1.27.8-v2", Namespace: testNs.Name}
			Eventually(func() error {
				return testEnv.Get(ctx, openstackNodeImageReleaseName, &openStackNodeImageRelease)
			}, timeout, interval).Should(BeNil())
		})

		It("sets OpenStackImageReadyCondition condition once node image is created and active", func() {
			Eventually(func() bool {
				openstackNodeImageReleaseName := types.NamespacedName{Name: "openstack-integration-1-27-ubuntu-test-image-v1.27.8-v2", Namespace: testNs.Name}
				return utils.IsPresentAndTrue(ctx, testEnv.Client, openstackNodeImageReleaseName, &openStackNodeImageRelease, cspov1alpha1.OpenStackImageReadyCondition)
			}, timeout, interval).Should(BeTrue())
		})

		It("sets OpenStackNodeImageRelease Status ready after node image is created and active", func() {
			Eventually(func() bool {
				openstackNodeImageReleaseName := types.NamespacedName{Name: "openstack-integration-1-27-ubuntu-test-image-v1.27.8-v2", Namespace: testNs.Name}
				if err := testEnv.Get(ctx, openstackNodeImageReleaseName, &openStackNodeImageRelease); err == nil {
					return openStackNodeImageRelease.Status.Ready
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("sets OpenStackClusterStackRelease Status ready after OpenStackNodeImageReleases is ready", func() {
			Eventually(func() bool {
				openStackClusterStackReleaseName := types.NamespacedName{Name: testClusterStackName, Namespace: testNs.Name}
				if err := testEnv.Get(ctx, openStackClusterStackReleaseName, openStackClusterStackRelease); err == nil {
					return openStackClusterStackRelease.Status.Ready
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})
	})
})
