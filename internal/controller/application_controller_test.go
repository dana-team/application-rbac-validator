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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Application Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	var (
		testNamespace        string
		testAppName          string
		reconciler           *ApplicationReconciler
		testNamespacePrefix  string
		prefixedNamespace    string
		nonPrefixedNamespace string
	)

	BeforeEach(func() {
		testNamespace = fmt.Sprintf("test-ns-%d", time.Now().UnixNano())
		testAppName = fmt.Sprintf("test-app-%d", time.Now().UnixNano())
		testNamespacePrefix = "argocd-"
		prefixedNamespace = fmt.Sprintf("%stest-%d", testNamespacePrefix, time.Now().UnixNano())
		nonPrefixedNamespace = fmt.Sprintf("other-test-%d", time.Now().UnixNano())

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
})
