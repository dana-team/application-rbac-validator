package handlers

import (
	"context"
	"testing"

	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dana-team/application-rbac-validator/internal/common"
	"github.com/dana-team/application-rbac-validator/internal/utils"
	testutils "github.com/dana-team/application-rbac-validator/test/utils"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testNamespace      = "test-namespace"
	testAppName        = "test-app"
	testDestNamespace  = "dest-namespace"
	testDestNamespace2 = "dest-namespace-2"
	testClusterServer  = "https://api.test-cluster.example.com:6443"
	testSecretName     = "test-cluster.example.com-cluster-secret"
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
			name: "ShouldAddNamespaceToSecretWhenNotPresent",
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
			name: "ShouldNotAddNamespaceWhenAlreadyPresent",
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
			name: "ShouldAppendNamespaceToExistingList",
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
			name: "ShouldSkipCluster-wideEnabledSecrets",
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
			testNS := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: testNamespace,
				},
			}
			cl := testutils.NewFakeClient(tc.app, tc.secret, testNS)

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
				actualNsList := utils.ExtractNamespacesFromSecret(updatedSecret)

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
			expectedNsList: []string{},
		},
		{
			name: "should keep namespace when other app uses it",
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
					common.NamespaceKey: []byte(testDestNamespace + "," + testDestNamespace2),
				},
			},
			expectError:    false,
			expectedNsList: []string{testDestNamespace2},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()

			_ = argoprojv1alpha1.AddToScheme(scheme)
			_ = corev1.AddToScheme(scheme)

			//nolint:prealloc
			objects := []client.Object{}
			objects = append(objects, tc.app, tc.secret)
			for _, app := range tc.otherApps {
				objects = append(objects, app)
			}

			cl := testutils.NewFakeClient(objects...)

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

			actualNsList := utils.ExtractNamespacesFromSecret(updatedSecret)
			if len(actualNsList) != len(tc.expectedNsList) {
				t.Errorf("expected namespace list %v but got %v", tc.expectedNsList, actualNsList)
			}

			for i, ns := range tc.expectedNsList {
				if i >= len(actualNsList) || actualNsList[i] != ns {
					t.Errorf("expected namespace at index %d to be %s but got %v", i, ns, actualNsList)
					break
				}
			}

		})
	}
}
