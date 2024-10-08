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
	"encoding/base64"
	"os"

	"github.com/SovereignCloudStack/cluster-stack-operator/pkg/test/utils"
	cspov1alpha1 "github.com/SovereignCloudStack/cluster-stack-provider-openstack/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apiv1alpha7 "sigs.k8s.io/cluster-api-provider-openstack/api/v1alpha7"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var _ = Describe("OpenStackClusterStackReleaseReconciler", func() {
	Context("test", func() {
		var (
			openStackClusterStackRelease    *cspov1alpha1.OpenStackClusterStackRelease
			testNs                          *corev1.Namespace
			openstackClusterStackReleaseKey types.NamespacedName
		)

		BeforeEach(func() {
			var err error
			testNs, err = testEnv.CreateNamespace(ctx, "oscsr-integration")
			Expect(err).NotTo(HaveOccurred())

			openstackClusterStackReleaseKey = types.NamespacedName{Name: "openstack-scs-1-28-v0-sha-umng5ie", Namespace: testNs.Name}

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
					Name:      "openstack-scs-1-28-v0-sha-umng5ie",
					Namespace: testNs.Name,
				},
				Spec: cspov1alpha1.OpenStackClusterStackReleaseSpec{
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
			Expect(os.RemoveAll(releaseDir)).To(Succeed())
			Eventually(func() error {
				return testEnv.Cleanup(ctx, testNs, openStackClusterStackRelease)
			}, timeout, interval).Should(BeNil())
		})

		It("creates the OpenStackNodeImageRelease object", func() {
			openstackNodeImageReleaseName := types.NamespacedName{Name: "openstack-scs-1-28-ubuntu-capi-image-v1.28.11-v0-sha-umng5ie", Namespace: testNs.Name}
			Eventually(func() error {
				var openStackNodeImageRelease cspov1alpha1.OpenStackNodeImageRelease
				return testEnv.Get(ctx, openstackNodeImageReleaseName, &openStackNodeImageRelease)
			}, timeout, interval).Should(BeNil())
		})

		It("sets ClusterStackReleaseAssetsReadyCondition condition once OpenStackClusterStackRelease object is created", func() {
			Eventually(func() bool {
				var openStackClusterStackRelease cspov1alpha1.OpenStackClusterStackRelease
				return utils.IsPresentAndTrue(ctx, testEnv.Client, openstackClusterStackReleaseKey, &openStackClusterStackRelease, cspov1alpha1.ClusterStackReleaseAssetsReadyCondition)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
