package handlers

import (
	"context"
	"testing"

	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dana-team/application-rbac-validator/internal/common"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	testNamespace      = "test-namespace"
	testAppName        = "test-app"
	testDestNamespace  = "dest-namespace"
	testDestNamespace2 = "dest-namespace-2"
	testClusterServer  = "https://api.test-cluster.example.com:6443"
	testSecretName     = "https://api.test-cluster.example.com:6443.example.com-cluster-tokentoken"
)

func TestHandleCreateOrUpdate(t *testing.T) {
	testCases := []struct {
		name           string
		app            *argoprojv1alpha1.Application
		secret         *corev1.Secret
		expectError    bool
		expectUpdate   bool
		expectedNsList []string
	}{
		{
			name: "should add namespace to secret when not present",
			app: &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAppName,
					Namespace: testNamespace,
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    testClusterServer,
						Namespace: testDestNamespace,
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSecretName,
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					common.NamespaceKey: []byte(""),
				},
			},
			expectError:    false,
			expectUpdate:   true,
			expectedNsList: []string{testDestNamespace},
		},
		{
			name: "should not add namespace when already present",
			app: &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAppName,
					Namespace: testNamespace,
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    testClusterServer,
						Namespace: testDestNamespace,
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSecretName,
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					common.NamespaceKey: []byte(testDestNamespace),
				},
			},
			expectError:    false,
			expectUpdate:   false,
			expectedNsList: []string{testDestNamespace},
		},
		{
			name: "should append namespace to existing list",
			app: &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAppName,
					Namespace: testNamespace,
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    testClusterServer,
						Namespace: testDestNamespace2,
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSecretName,
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					common.NamespaceKey: []byte(testDestNamespace),
				},
			},
			expectError:    false,
			expectUpdate:   true,
			expectedNsList: []string{testDestNamespace, testDestNamespace2},
		},
		{
			name: "should skip in-cluster applications",
			app: &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAppName,
					Namespace: testNamespace,
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    common.InClusterValues[0],
						Namespace: testDestNamespace,
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSecretName,
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					common.NamespaceKey: []byte(""),
				},
			},
			expectError:  false,
			expectUpdate: false,
		},
		{
			name: "should skip cluster-wide enabled secrets",
			app: &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAppName,
					Namespace: testNamespace,
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    testClusterServer,
						Namespace: testDestNamespace,
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSecretName,
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					common.NamespaceKey:        []byte(""),
					common.ClusterResourcesKey: []byte("true"),
				},
			},
			expectError:  false,
			expectUpdate: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			common.ServerUrlDomain = "example.com"
			scheme := runtime.NewScheme()
			_ = argoprojv1alpha1.AddToScheme(scheme)
			_ = corev1.AddToScheme(scheme)

			cl := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.app, tc.secret).
				Build()

			log := logr.Discard()
			ctx := context.Background()

			err := HandleCreateOrUpdate(log, ctx, cl, tc.app)

			if tc.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			updatedSecret := &corev1.Secret{}
			err = cl.Get(ctx, types.NamespacedName{Name: tc.secret.Name, Namespace: tc.secret.Namespace}, updatedSecret)
			if err != nil {
				t.Fatalf("failed to get secret: %v", err)
			}

			if tc.expectUpdate && tc.expectedNsList != nil {
				actualNsList := common.GetNamespacesList(updatedSecret)

				expectedNsSet := make(map[string]bool)
				for _, ns := range tc.expectedNsList {
					if ns != "" {
						expectedNsSet[ns] = true
					}
				}

				actualNsSet := make(map[string]bool)
				for _, ns := range actualNsList {
					if ns != "" {
						actualNsSet[ns] = true
					}
				}

				if len(expectedNsSet) != len(actualNsSet) {
					t.Errorf("expected namespace set %v but got %v", expectedNsSet, actualNsSet)
				} else {
					for ns := range expectedNsSet {
						if !actualNsSet[ns] {
							t.Errorf("expected namespace %s not found in actual set %v", ns, actualNsSet)
						}
					}
				}
			}

			updatedApp := &argoprojv1alpha1.Application{}
			err = cl.Get(ctx, types.NamespacedName{Name: tc.app.Name, Namespace: tc.app.Namespace}, updatedApp)
			if err != nil {
				t.Fatalf("failed to get application: %v", err)
			}

			if !controllerutil.ContainsFinalizer(updatedApp, common.FinalizerName) && !common.IsInCluster(tc.app.Spec.Destination.Server) {
				nsList := common.GetNamespacesList(tc.secret)
				clusterWideRaw, ok := tc.secret.Data[common.ClusterResourcesKey]
				clusterWide := ok && string(clusterWideRaw) == "true"
				if !clusterWide && len(nsList) > 0 {
					t.Errorf("expected finalizer to be added")
				}
			}
		})
	}
}

func TestHandleDelete(t *testing.T) {
	testCases := []struct {
		name           string
		app            *argoprojv1alpha1.Application
		otherApps      []*argoprojv1alpha1.Application
		secret         *corev1.Secret
		expectError    bool
		expectedNsList []string
	}{
		{
			name: "should remove namespace when no other app uses it",
			app: &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:       testAppName,
					Namespace:  testNamespace,
					Finalizers: []string{common.FinalizerName},
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    testClusterServer,
						Namespace: testDestNamespace,
					},
				},
			},
			otherApps: []*argoprojv1alpha1.Application{},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSecretName,
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					common.NamespaceKey: []byte(testDestNamespace),
				},
			},
			expectError:    false,
			expectedNsList: []string{},
		},
		{
			name: "should keep namespace when other app uses it",
			app: &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:       testAppName,
					Namespace:  testNamespace,
					Finalizers: []string{common.FinalizerName},
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    testClusterServer,
						Namespace: testDestNamespace,
					},
				},
			},
			otherApps: []*argoprojv1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-app",
						Namespace: testNamespace,
					},
					Spec: argoprojv1alpha1.ApplicationSpec{
						Destination: argoprojv1alpha1.ApplicationDestination{
							Server:    testClusterServer,
							Namespace: testDestNamespace,
						},
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSecretName,
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					common.NamespaceKey: []byte(testDestNamespace),
				},
			},
			expectError:    false,
			expectedNsList: []string{testDestNamespace},
		},
		{
			name: "should remove only target namespace from list",
			app: &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:       testAppName,
					Namespace:  testNamespace,
					Finalizers: []string{common.FinalizerName},
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    testClusterServer,
						Namespace: testDestNamespace,
					},
				},
			},
			otherApps: []*argoprojv1alpha1.Application{},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSecretName,
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					common.NamespaceKey: []byte(testDestNamespace + "," + testDestNamespace2),
				},
			},
			expectError:    false,
			expectedNsList: []string{testDestNamespace2},
		},
		{
			name: "should do nothing if no finalizer present",
			app: &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAppName,
					Namespace: testNamespace,
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    testClusterServer,
						Namespace: testDestNamespace,
					},
				},
			},
			otherApps: []*argoprojv1alpha1.Application{},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSecretName,
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					common.NamespaceKey: []byte(testDestNamespace),
				},
			},
			expectError:    false,
			expectedNsList: []string{testDestNamespace},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			common.ServerUrlDomain = "example.com"
			scheme := runtime.NewScheme()
			_ = argoprojv1alpha1.AddToScheme(scheme)
			_ = corev1.AddToScheme(scheme)

			objects := []client.Object{tc.app, tc.secret}
			for _, app := range tc.otherApps {
				objects = append(objects, app)
			}

			cl := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			log := logr.Discard()
			ctx := context.Background()

			err := HandleDelete(log, ctx, cl, tc.app)

			if tc.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			updatedSecret := &corev1.Secret{}
			err = cl.Get(ctx, types.NamespacedName{Name: tc.secret.Name, Namespace: tc.secret.Namespace}, updatedSecret)
			if err != nil {
				t.Fatalf("failed to get secret: %v", err)
			}

			actualNsList := common.GetNamespacesList(updatedSecret)
			if len(actualNsList) != len(tc.expectedNsList) {
				t.Errorf("expected namespace list %v but got %v", tc.expectedNsList, actualNsList)
			}

			for i, ns := range tc.expectedNsList {
				if i >= len(actualNsList) || actualNsList[i] != ns {
					t.Errorf("expected namespace at index %d to be %s but got %v", i, ns, actualNsList)
					break
				}
			}

			if controllerutil.ContainsFinalizer(tc.app, common.FinalizerName) {
				updatedApp := &argoprojv1alpha1.Application{}
				err = cl.Get(ctx, types.NamespacedName{Name: tc.app.Name, Namespace: tc.app.Namespace}, updatedApp)
				if err != nil {
					t.Fatalf("failed to get application: %v", err)
				}

				if controllerutil.ContainsFinalizer(updatedApp, common.FinalizerName) {
					t.Errorf("expected finalizer to be removed")
				}
			}
		})
	}
}
