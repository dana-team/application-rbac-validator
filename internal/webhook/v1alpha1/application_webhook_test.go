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

package v1alpha1

import (
	"context"
	"fmt"
	"os"

	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dana-team/application-rbac-validator/internal/common"
	"github.com/dana-team/application-rbac-validator/internal/utils"
	testutils "github.com/dana-team/application-rbac-validator/test/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("application-rbac-validator Webhook", func() {
	Context("On Application validation", func() {
		ctx := context.Background()

		common.WebhookNamespacePath = testutils.WebhookNamespaceTestPath
		common.ServerUrlDomain = "example.com"
		common.DomainEnvVarFound = true

		var (
			testNamespace string
			resourceName  string
			testValidator ApplicationCustomValidator
		)

		BeforeEach(func() {
			testValidator = ApplicationCustomValidator{Client: k8sClient,
				destinationClusterClient: testutils.NewMockedDestinationClusterClient()}
			Expect(testValidator).NotTo(BeNil(), "Expected validator to be initialized")

			resourceName = fmt.Sprintf("test-resource-%s", testutils.GenerateRandomSuffix(6))
			testNamespace = fmt.Sprintf("test-ns-%s", testutils.GenerateRandomSuffix(6))

			By("creating the test namespace")
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: testNamespace,
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			By("creating a file that stores the webhook's current namespace name")
			Expect(os.MkdirAll(testutils.WebhookNamespaceDir, 0755)).To(Succeed())

			Expect(os.WriteFile(testutils.WebhookNamespaceTestPath, []byte(testNamespace), 0644)).To(Succeed())
		})
		AfterEach(func() {
			By("cleaning up the test namespace")
			ns := &corev1.Namespace{}
			nsErr := k8sClient.Get(ctx, types.NamespacedName{Name: testNamespace}, ns)

			if nsErr == nil || !errors.IsNotFound(nsErr) {
				Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
			}

		})

		Context("When creating and updating an Application", func() {
			for _, tc := range testCases {
				if tc.name != "should allow valid Application" {
					return
				}
				t := tc
				It(t.name, func() {
					By(fmt.Sprintf("Creating the Application for test: %s", t.name))
					application := &argoprojv1alpha1.Application{
						ObjectMeta: metav1.ObjectMeta{
							Name:      resourceName,
							Namespace: testNamespace,
						},
						Spec: t.spec,
					}

					By("creating the argo instance ConfigMap for testing")
					configMap := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      common.ArgoInstanceConfigMapName,
							Namespace: testNamespace,
						},
						Data: map[string]string{
							t.argoInstanceUsersConfigMapKey: t.argoInstanceUsersConfigMapData,
							t.argoInstanceNameConfigMapKey:  testutils.ArgoInstanceNameConfigMapData,
						},
					}
					Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

					By("creating the ConfigMap that stores the destination server token")
					var tokenPath string
					if t.spec.Destination.Name != "" {
						// For destination name, use the format: name-domain-port-token
						tokenPath = fmt.Sprintf("%s-%s-%s-token", t.serverTokenKey, common.ServerUrlDomain, common.DefaultServerUrlPort)
					} else {
						// For destination server URL, format it as file-safe name
						tokenPath = utils.FormatFileSafeServerURL(t.serverTokenKey) + "-token"
					}

					configMap = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      common.ClusterTokensConfigMapName,
							Namespace: testNamespace,
						},
						Data: map[string]string{
							tokenPath: "dummy-token-content",
						},
					}
					Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

					if t.bypassLabel != "" {
						By("adding bypass label to the Application's namespace")
						ns := &corev1.Namespace{}
						Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testNamespace}, ns)).To(Succeed())

						if ns.Labels == nil {
							ns.Labels = make(map[string]string)
						}
						ns.Labels[t.bypassLabel] = "true"

						Expect(k8sClient.Update(ctx, ns)).To(Succeed())
					}

					if t.isManagementApplication {
						By("creating management application with correct name")
						application.Name = testutils.ArgoInstanceNameConfigMapData + "-mgmt"
					}

					By(fmt.Sprintf("starting create validation test: %s", t.name))
					_, err := testValidator.ValidateCreate(ctx, application)

					if !t.expectToSucceed {
						Expect(err).To(HaveOccurred())
					} else {
						Expect(err).ToNot(HaveOccurred())
					}

					By(fmt.Sprintf("starting update validation test: %s", t.name))
					oldApplication := &argoprojv1alpha1.Application{
						ObjectMeta: metav1.ObjectMeta{
							Name:      resourceName,
							Namespace: testNamespace,
						},
						Spec: argoprojv1alpha1.ApplicationSpec{Project: "an empty application"},
					}

					_, err = testValidator.ValidateUpdate(ctx, oldApplication, application)

					if !t.expectToSucceed {
						Expect(err).To(HaveOccurred())
					} else {
						Expect(err).ToNot(HaveOccurred())
					}
				})
			}
		})
	})
})
