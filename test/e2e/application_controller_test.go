/*
Copyright 2025.

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

package e2e

import (
	"context"
	"fmt"
	"strings"

	"github.com/dana-team/application-rbac-validator/internal/common"
	testutils "github.com/dana-team/application-rbac-validator/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Application Controller", func() {

	Context("When reconciling an Application resource", func() {
		var namespaceName string
		var secretName = fmt.Sprintf("%s-cluster-secret", testutils.ClusterHostname)
		BeforeEach(func() {
			namespaceName = fmt.Sprintf("test-ns-%s", testutils.GenerateRandomSuffix(6))
			Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
					Labels: map[string]string{
						// Bypass webhook since tests are for controller
						testutils.AdminBypassLabel: "true",
					},
				},
			})).To(Succeed())

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespaceName,
				},
				StringData: map[string]string{
					"namespaces": "",
				},
			}
			Expect(k8sClient.Create(context.Background(), secret)).To(Succeed())

		})
		AfterEach(func() {
			By("Cleaning up the created namespace")
			Expect(k8sClient.Delete(context.Background(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
				},
			})).To(Succeed())
		})
		It(" should add a namespace to the secret when an application is created", func() {
			By("Creating an Application resource")
			application := testutils.GenerateTestApplication(
				namespaceName,
				testutils.TestDestinationServerUrl,
				testutils.TestDestinationNamespace,
			)
			Expect(k8sClient.Create(context.Background(), application)).To(Succeed())

			By("Verifying the namespace is added to the secret")
			secret := &corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      secretName,
					Namespace: namespaceName,
				}, secret)
				if err != nil {
					return false
				}
				namespaces, exists := secret.Data["namespaces"]
				if !exists {
					return false
				}
				return strings.Contains(string(namespaces), testutils.TestDestinationNamespace)
			}, testutils.DefaultTimeout, testutils.DefaultInterval).Should(BeTrue())

		})
		It("Should not add a namespace when a cluster has cluster-wide enabled", func() {
			By("Updating the cluster secret to be cluster-wide")
			secret := &corev1.Secret{}
			Expect(k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      secretName,
				Namespace: namespaceName,
			}, secret)).To(Succeed())
			secret.Data[common.ClusterResourcesKey] = []byte("true")
			Expect(k8sClient.Update(context.Background(), secret)).To(Succeed())
			By("Creating an Application resource")
			application := testutils.GenerateTestApplication(
				namespaceName,
				testutils.TestDestinationServerUrl,
				testutils.TestDestinationNamespace,
			)
			Expect(k8sClient.Create(context.Background(), application)).To(Succeed())

			By("Verifying the namespace is not added to the secret")
			secret = &corev1.Secret{}
			Consistently(func() bool {
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      secretName,
					Namespace: namespaceName,
				}, secret)
				if err != nil {
					return false
				}
				namespaces, exists := secret.Data["namespaces"]
				if !exists {
					return true
				}
				return !strings.Contains(string(namespaces), testutils.TestDestinationNamespace)
			}, testutils.DefaultTimeout/4, testutils.DefaultInterval).Should(BeTrue())
		})
		It("Should not optimize application with the in-cluster destination", func() {
			By("Creating an Application resource with in-cluster destination")
			application := testutils.GenerateTestApplication(
				namespaceName,
				"in-cluster",
				namespaceName,
			)
			Expect(k8sClient.Create(context.Background(), application)).To(Succeed())
			By("verifying the namespace is not added to the secret")
			Eventually(func() bool {
				secret := &corev1.Secret{}
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      secretName,
					Namespace: namespaceName,
				}, secret)
				if err != nil {
					return false
				}
				namespaces, exists := secret.Data["namespaces"]
				if !exists {
					return true
				}
				return !strings.Contains(string(namespaces), namespaceName)
			}, testutils.DefaultTimeout/4, testutils.DefaultInterval).Should(BeTrue())

		})

		It("Should handle multiple applications with the same destination namespace", func() {
			By("Creating the first Application resource")
			application1 := testutils.GenerateTestApplication(
				namespaceName,
				testutils.TestDestinationServerUrl,
				testutils.TestDestinationNamespace,
			)
			Expect(k8sClient.Create(context.Background(), application1)).To(Succeed())
			By("Creating the second Application resource")
			application2 := testutils.GenerateTestApplication(
				namespaceName,
				testutils.TestDestinationServerUrl,
				testutils.TestDestinationNamespace,
			)
			Expect(k8sClient.Create(context.Background(), application2)).To(Succeed())

			By("Verifying the namespace is added to the secret one time")
			foundSecret := &corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      secretName,
					Namespace: namespaceName,
				}, foundSecret)
				if err != nil {
					return false
				}
				namespaces, exists := foundSecret.Data["namespaces"]
				if !exists {
					return true
				}
				return string(namespaces) == testutils.TestDestinationNamespace
			}, testutils.DefaultTimeout/4, testutils.DefaultInterval).Should(BeTrue(),
				fmt.Sprintf(
					"Expected namespaces key in secret to be %s, got: %s",
					testutils.TestDestinationNamespace,
					string(foundSecret.Data["namespaces"])),
			)
		})

		It("Should ignore applications in namespaces without prefix", func() {
			By("Creating a new namespace and secret without the prefix")
			nonPrefixedNamespace := fmt.Sprintf("non-prefixed-ns-%s", testutils.GenerateRandomSuffix(6))
			Expect(k8sClient.Create(context.Background(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: nonPrefixedNamespace,
					Labels: map[string]string{
						// Bypass webhook since tests are for controller
						testutils.AdminBypassLabel: "true",
					},
				},
			})).To(Succeed())
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: nonPrefixedNamespace,
				},
				StringData: map[string]string{},
			}
			Expect(k8sClient.Create(context.Background(), secret)).To(Succeed())

			defer func() {
				By("Cleaning up the created namespace")
				Expect(k8sClient.Delete(context.Background(), &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: nonPrefixedNamespace,
					},
				})).To(Succeed())
			}()

			By("Creating an Application resource in the non-prefixed namespace")
			application := testutils.GenerateTestApplication(
				nonPrefixedNamespace,
				testutils.TestDestinationServerUrl,
				testutils.TestDestinationNamespace,
			)
			Expect(k8sClient.Create(context.Background(), application)).To(Succeed())
			By("Verifying the namespace is not added to the secret")
			Eventually(func() bool {
				foundSecret := &corev1.Secret{}
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      secretName,
					Namespace: nonPrefixedNamespace,
				}, foundSecret)
				if err != nil {
					return false
				}
				namespaces, exists := foundSecret.Data["namespaces"]
				if !exists {
					return true
				}
				return !strings.Contains(string(namespaces), testutils.TestDestinationNamespace)
			}, testutils.DefaultTimeout/4, testutils.DefaultInterval).Should(BeTrue(),
				fmt.Sprintf("Expected namespaces key in secret to be empty, got: %s", string(secret.Data["namespaces"])),
			)
		})
	})
})
