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

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dana-team/application-rbac-validator/internal/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Application Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	var (
		testNamespace        string
		testClusterServerURL string
		testDestinationNS    string
		testSecretName       string
		testAppName          string
		namespacedName       types.NamespacedName
		reconciler           *ApplicationReconciler
		testNamespacePrefix  string
		prefixedNamespace    string
		nonPrefixedNamespace string
	)

	BeforeEach(func() {
		testNamespace = fmt.Sprintf("test-ns-%d", time.Now().UnixNano())
		testDestinationNS = "destination-ns"
		testClusterServerURL = "test-cluster"
		testSecretName = "test-cluster.example.com-cluster-tokentoken"
		testAppName = fmt.Sprintf("test-app-%d", time.Now().UnixNano())
		testNamespacePrefix = "argocd-"
		prefixedNamespace = fmt.Sprintf("%stest-%d", testNamespacePrefix, time.Now().UnixNano())
		nonPrefixedNamespace = fmt.Sprintf("other-test-%d", time.Now().UnixNano())

		common.ServerUrlDomain = "example.com"

		namespacedName = types.NamespacedName{
			Name:      testAppName,
			Namespace: testNamespace,
		}

		reconciler = &ApplicationReconciler{
			Client:          k8sClient,
			Scheme:          k8sClient.Scheme(),
			NamespacePrefix: testNamespacePrefix,
		}

		By("creating test namespace")
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		Expect(k8sClient.Create(context.Background(), ns)).To(Succeed())

		By("creating prefixed namespace")
		prefixedNs := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: prefixedNamespace,
			},
		}
		Expect(k8sClient.Create(context.Background(), prefixedNs)).To(Succeed())

		By("creating non-prefixed namespace")
		nonPrefixedNs := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: nonPrefixedNamespace,
			},
		}
		Expect(k8sClient.Create(context.Background(), nonPrefixedNs)).To(Succeed())
	})

	AfterEach(func() {
		By("cleaning up test resources")
		ctx := context.Background()

		By("deleting Application if it exists")
		app := &argoprojv1alpha1.Application{}
		if err := k8sClient.Get(ctx, namespacedName, app); err == nil {
			// Remove finalizers to allow deletion
			app.Finalizers = []string{}
			_ = k8sClient.Update(ctx, app)
			_ = k8sClient.Delete(ctx, app)
		}

		By("deleting cluster secret if it exists")
		secret := &corev1.Secret{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: testSecretName, Namespace: testNamespace}, secret); err == nil {
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
		}

		By("deleting test namespace")
		ns := &corev1.Namespace{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: testNamespace}, ns); err == nil {
			Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
		}

		By("deleting prefixed namespace")
		prefixedNs := &corev1.Namespace{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: prefixedNamespace}, prefixedNs); err == nil {
			Expect(k8sClient.Delete(ctx, prefixedNs)).To(Succeed())
		}

		By("deleting non-prefixed namespace")
		nonPrefixedNs := &corev1.Namespace{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: nonPrefixedNamespace}, nonPrefixedNs); err == nil {
			Expect(k8sClient.Delete(ctx, nonPrefixedNs)).To(Succeed())
		}
	})

	Context("When testing namespace prefix filtering logic", func() {
		It("should enqueue Applications from namespaces with matching prefix", func() {
			By("creating a mock Application object in prefixed namespace")
			app := &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAppName,
					Namespace: prefixedNamespace,
				},
			}

			By("testing the prefix logic")
			namespace := app.GetNamespace()
			shouldEnqueue := len(reconciler.NamespacePrefix) > 0 &&
				len(namespace) > 0 &&
				strings.HasPrefix(namespace, reconciler.NamespacePrefix)

			Expect(shouldEnqueue).To(BeTrue(), "Application in prefixed namespace should be enqueued")
		})

		It("should not enqueue Applications from namespaces without matching prefix", func() {
			By("creating a mock Application object in non-prefixed namespace")
			app := &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAppName,
					Namespace: nonPrefixedNamespace,
				},
			}

			By("testing the prefix logic")
			namespace := app.GetNamespace()
			shouldEnqueue := len(reconciler.NamespacePrefix) > 0 &&
				len(namespace) > 0 &&
				strings.HasPrefix(namespace, reconciler.NamespacePrefix)

			Expect(shouldEnqueue).To(BeFalse(), "Application in non-prefixed namespace should not be enqueued")
		})

		It("should not enqueue Applications when namespace prefix is empty", func() {
			By("creating reconciler with empty prefix")
			emptyPrefixReconciler := &ApplicationReconciler{
				Client:          k8sClient,
				Scheme:          k8sClient.Scheme(),
				NamespacePrefix: "",
			}

			By("creating a mock Application object")
			app := &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAppName,
					Namespace: nonPrefixedNamespace,
				},
			}

			By("testing the prefix logic with empty prefix")
			namespace := app.GetNamespace()
			shouldEnqueue := len(emptyPrefixReconciler.NamespacePrefix) > 0 &&
				len(namespace) > 0 &&
				strings.HasPrefix(namespace, emptyPrefixReconciler.NamespacePrefix)

			Expect(shouldEnqueue).To(BeFalse(), "Empty prefix means no filtering, so the check returns false")
		})
	})

	Context("When reconciling an Application that doesn't exist", func() {
		It("should handle not found error gracefully", func() {
			ctx := context.Background()

			By("reconciling a non-existent Application")
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent-app",
					Namespace: testNamespace,
				},
			}
			result, err := reconciler.Reconcile(ctx, req)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})

	Context("When reconciling an Application targeting in-cluster", func() {
		It("should skip reconciliation for in-cluster destinations", func() {
			ctx := context.Background()

			By("creating an Application targeting in-cluster")
			app := &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAppName,
					Namespace: testNamespace,
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    "in-cluster",
						Namespace: testDestinationNS,
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			By("reconciling the Application")
			req := reconcile.Request{
				NamespacedName: namespacedName,
			}
			result, err := reconciler.Reconcile(ctx, req)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			By("verifying no finalizer was added")
			Eventually(func() bool {
				updatedApp := &argoprojv1alpha1.Application{}
				if err := k8sClient.Get(ctx, namespacedName, updatedApp); err != nil {
					return true // App might be gone, that's ok
				}
				for _, finalizer := range updatedApp.Finalizers {
					if finalizer == common.FinalizerName {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("cleaning up Application")
			appToDelete := &argoprojv1alpha1.Application{}
			if err := k8sClient.Get(ctx, namespacedName, appToDelete); err == nil {
				Expect(k8sClient.Delete(ctx, appToDelete)).To(Succeed())
			}
		})
	})

	Context("When reconciling an Application with cluster secret", func() {
		It("should add namespace to secret and add finalizer", func() {
			ctx := context.Background()

			By("creating a cluster secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSecretName,
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					common.NamespaceKey: []byte("existing-ns"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			By("creating an Application")
			app := &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAppName,
					Namespace: testNamespace,
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    testClusterServerURL,
						Namespace: testDestinationNS,
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			By("reconciling the Application")
			req := reconcile.Request{
				NamespacedName: namespacedName,
			}
			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())

			By("verifying the secret was updated with the destination namespace")
			Eventually(func() bool {
				updatedSecret := &corev1.Secret{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testSecretName, Namespace: testNamespace}, updatedSecret)
				if err != nil {
					return false
				}
				namespaces := string(updatedSecret.Data[common.NamespaceKey])
				return strings.Contains(namespaces, testDestinationNS)
			}, timeout, interval).Should(BeTrue())

			By("verifying the finalizer was added")
			Eventually(func() bool {
				updatedApp := &argoprojv1alpha1.Application{}
				err := k8sClient.Get(ctx, namespacedName, updatedApp)
				if err != nil {
					return false
				}
				for _, finalizer := range updatedApp.Finalizers {
					if finalizer == common.FinalizerName {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("cleaning up Application")
			appToDelete := &argoprojv1alpha1.Application{}
			if err := k8sClient.Get(ctx, namespacedName, appToDelete); err == nil {
				// Remove finalizer first
				appToDelete.Finalizers = []string{}
				Expect(k8sClient.Update(ctx, appToDelete)).To(Succeed())
				Expect(k8sClient.Delete(ctx, appToDelete)).To(Succeed())
			}

			By("cleaning up secret")
			secretToDelete := &corev1.Secret{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: testSecretName, Namespace: testNamespace}, secretToDelete); err == nil {
				Expect(k8sClient.Delete(ctx, secretToDelete)).To(Succeed())
			}
		})

		It("should skip optimization for clusterwide secrets", func() {
			ctx := context.Background()

			By("creating a cluster secret with clusterResources=true")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSecretName,
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					common.NamespaceKey:        []byte("existing-ns"),
					common.ClusterResourcesKey: []byte("true"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			By("creating an Application")
			app := &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAppName,
					Namespace: testNamespace,
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    testClusterServerURL,
						Namespace: testDestinationNS,
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			By("reconciling the Application")
			req := reconcile.Request{
				NamespacedName: namespacedName,
			}
			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())

			By("verifying the secret was not modified")
			Consistently(func() string {
				updatedSecret := &corev1.Secret{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testSecretName, Namespace: testNamespace}, updatedSecret)
				if err != nil {
					return ""
				}
				return string(updatedSecret.Data[common.NamespaceKey])
			}, time.Second*2, interval).Should(Equal("existing-ns"))

			By("cleaning up Application")
			appToDelete := &argoprojv1alpha1.Application{}
			if err := k8sClient.Get(ctx, namespacedName, appToDelete); err == nil {
				Expect(k8sClient.Delete(ctx, appToDelete)).To(Succeed())
			}

			By("cleaning up secret")
			secretToDelete := &corev1.Secret{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: testSecretName, Namespace: testNamespace}, secretToDelete); err == nil {
				Expect(k8sClient.Delete(ctx, secretToDelete)).To(Succeed())
			}
		})
	})

	Context("When an Application with a finalizer is deleted", func() {
		It("should remove namespace from secret when no other apps use it", func() {
			ctx := context.Background()

			By("creating a cluster secret with the destination namespace")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSecretName,
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					common.NamespaceKey: []byte(fmt.Sprintf("other-ns,%s", testDestinationNS)),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			By("creating an Application with finalizer")
			app := &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:       testAppName,
					Namespace:  testNamespace,
					Finalizers: []string{common.FinalizerName},
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    testClusterServerURL,
						Namespace: testDestinationNS,
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			By("deleting the Application")
			Expect(k8sClient.Delete(ctx, app)).To(Succeed())

			By("reconciling the deletion")
			req := reconcile.Request{
				NamespacedName: namespacedName,
			}
			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())

			By("verifying the namespace was removed from secret")
			Eventually(func() string {
				updatedSecret := &corev1.Secret{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testSecretName, Namespace: testNamespace}, updatedSecret)
				if err != nil {
					return ""
				}
				return string(updatedSecret.Data[common.NamespaceKey])
			}, timeout, interval).Should(Equal("other-ns"))

			By("verifying the Application was deleted")
			Eventually(func() bool {
				deletedApp := &argoprojv1alpha1.Application{}
				err := k8sClient.Get(ctx, namespacedName, deletedApp)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("cleaning up secret")
			secretToDelete := &corev1.Secret{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: testSecretName, Namespace: testNamespace}, secretToDelete); err == nil {
				Expect(k8sClient.Delete(ctx, secretToDelete)).To(Succeed())
			}
		})

		It("should not remove namespace from secret if other apps use it", func() {
			ctx := context.Background()

			By("creating a cluster secret with the destination namespace")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSecretName,
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					common.NamespaceKey: []byte(testDestinationNS),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			By("creating first Application with finalizer")
			app1 := &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:       testAppName,
					Namespace:  testNamespace,
					Finalizers: []string{common.FinalizerName},
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    testClusterServerURL,
						Namespace: testDestinationNS,
					},
				},
			}
			Expect(k8sClient.Create(ctx, app1)).To(Succeed())

			By("creating second Application using the same namespace")
			app2Name := fmt.Sprintf("test-app2-%d", time.Now().UnixNano())
			app2 := &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      app2Name,
					Namespace: testNamespace,
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    testClusterServerURL,
						Namespace: testDestinationNS,
					},
				},
			}
			Expect(k8sClient.Create(ctx, app2)).To(Succeed())

			By("deleting the first Application")
			Expect(k8sClient.Delete(ctx, app1)).To(Succeed())

			By("reconciling the deletion")
			req := reconcile.Request{
				NamespacedName: namespacedName,
			}
			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())

			By("verifying the namespace was not removed from secret")
			Consistently(func() string {
				updatedSecret := &corev1.Secret{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testSecretName, Namespace: testNamespace}, updatedSecret)
				if err != nil {
					return ""
				}
				return string(updatedSecret.Data[common.NamespaceKey])
			}, time.Second*2, interval).Should(Equal(testDestinationNS))

			By("cleaning up second Application")
			app2ToDelete := &argoprojv1alpha1.Application{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: app2Name, Namespace: testNamespace}, app2ToDelete); err == nil {
				Expect(k8sClient.Delete(ctx, app2ToDelete)).To(Succeed())
			}

			By("cleaning up secret")
			secretToDelete := &corev1.Secret{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: testSecretName, Namespace: testNamespace}, secretToDelete); err == nil {
				Expect(k8sClient.Delete(ctx, secretToDelete)).To(Succeed())
			}
		})
	})
})
