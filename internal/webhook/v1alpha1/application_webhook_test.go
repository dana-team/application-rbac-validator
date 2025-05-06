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
	"time"

	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/dana-team/application-rbac-validator/internal/webhook/common"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("application-rbac-validator Webhook", func() {
	Context("On Application validation", func() {
		ctx := context.Background()

		common.WebhookNamespacePath = webhookNamespaceTestPath

		var testNamespace = ""

		BeforeEach(func() {
			validator = ApplicationCustomValidator{Client: k8sClient,
				destinationClusterClient: NewMockedDestinationClusterClient()}

			Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")

			resourceName := fmt.Sprintf("test-resource-%d", time.Now().UnixNano())
			testNamespace = fmt.Sprintf("test-ns-%d", time.Now().UnixNano())
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespace,
			}

			By("creating the test namespace")
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: typeNamespacedName.Namespace,
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			By("creating a file that stores the webhook's current namespace name")
			Expect(os.MkdirAll(webhookNamespaceDir, 0755)).To(Succeed())

			Expect(os.WriteFile(webhookNamespaceTestPath, []byte(typeNamespacedName.Namespace), 0644)).To(Succeed())
		})
		AfterEach(func() {
			By("cleaning up the test namespace")
			ns := &corev1.Namespace{}
			nsErr := k8sClient.Get(ctx, types.NamespacedName{Name: typeNamespacedName.Namespace}, ns)

			if nsErr == nil || !errors.IsNotFound(nsErr) {
				Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
			}

			By("deleting the file that stores the webhook's current namespace name")
			_ = os.Remove(webhookNamespaceTestPath)
		})

		Context("When creating and updating an Application", func() {
			for _, tc := range testCases {
				t := tc
				It(t.name, func() {
					By(fmt.Sprintf("Creating the Application for test: %s", t.name))
					application := &argoprojv1alpha1.Application{
						ObjectMeta: metav1.ObjectMeta{
							Name:      typeNamespacedName.Name,
							Namespace: typeNamespacedName.Namespace,
						},
						Spec: t.spec,
					}

					By("creating the argo instance ConfigMap for testing")
					configMap := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      common.ArgoInstanceConfigMapName,
							Namespace: typeNamespacedName.Namespace,
						},
						Data: map[string]string{
							t.argoInstanceUsersConfigMapKey: t.argoInstanceUsersConfigMapData,
							t.argoInstanceNameConfigMapKey:  ArgoInstanceNameConfigMapData,
						},
					}
					Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

					By("creating the ConfigMap that stores the destination server token")
					tokenPath := common.FormatServerURL(t.serverTokenKey) + "-token"

					configMap = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      common.ClusterTokensConfigMapName,
							Namespace: typeNamespacedName.Namespace,
						},
						Data: map[string]string{
							tokenPath: "dummy-token-content",
						},
					}
					Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

					if t.isBypassLabelExists {
						By("adding bypass label to the Application's namespace")
						ns := &corev1.Namespace{}
						Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testNamespace}, ns)).To(Succeed())

						if ns.Labels == nil {
							ns.Labels = make(map[string]string)
						}
						ns.Labels[common.AdminBypassLabel] = "true"

						Expect(k8sClient.Update(ctx, ns)).To(Succeed())
					}

					if t.isManagementApplication {
						By("creating management application with correct name")
						application.Name = ArgoInstanceNameConfigMapData + "-mgmt"
					}

					By(fmt.Sprintf("starting create validation test: %s", t.name))
					_, err := validator.ValidateCreate(ctx, application)

					if !t.expectToSucceed {
						Expect(err).To(HaveOccurred())
					} else {
						Expect(err).ToNot(HaveOccurred())
					}

					By(fmt.Sprintf("starting update validation test: %s", t.name))
					_, err = validator.ValidateUpdate(ctx, application, application)

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
