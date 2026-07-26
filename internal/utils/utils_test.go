package utils

import (
	"context"
	"os"
	"strings"
	"testing"

	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dana-team/application-rbac-validator/internal/common"
	testutils "github.com/dana-team/application-rbac-validator/test/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	sampleServerName          = "test-server"
	sampleNamespaceName       = "test-namespace"
	sampleNamespaceObjectName = "test-ns"
	sampleClusterName         = "my-cluster"
	inClusterAlias            = "in-cluster"
	inClusterServerURL        = "kubernetes.svc.cluster.local"
	sampleFQDNServerURL       = "https://api.my-cluster.domain.example.com:6443"
	sampleClusterServerURL    = "https://api.my-cluster.example.com:6443"
	sampleArgoInstanceName    = "argo-instance"
	sampleUser                = "user1"
	sampleArgoCDNamespace     = "argocd"
)

func TestIsNotSpecUpdate(t *testing.T) {
	testCases := []struct {
		name     string
		oldApp   *argoprojv1alpha1.Application
		newApp   *argoprojv1alpha1.Application
		expected bool
	}{
		{
			name: "should return true when specs are identical",
			oldApp: &argoprojv1alpha1.Application{
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    sampleServerName,
						Namespace: sampleNamespaceName,
					},
				},
			},
			newApp: &argoprojv1alpha1.Application{
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    sampleServerName,
						Namespace: sampleNamespaceName,
					},
				},
			},
			expected: true,
		},
		{
			name: "should return false when specs are different",
			oldApp: &argoprojv1alpha1.Application{
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    sampleServerName,
						Namespace: sampleNamespaceName,
					},
				},
			},
			newApp: &argoprojv1alpha1.Application{
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    sampleServerName,
						Namespace: "different-namespace",
					},
				},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsNotSpecUpdate(tc.oldApp, tc.newApp)
			if result != tc.expected {
				t.Errorf("expected %v but got %v", tc.expected, result)
			}
		})
	}
}

func TestValidateServerUrlFormat(t *testing.T) {
	testCases := []struct {
		name     string
		server   string
		expected bool
	}{
		{
			name:     "should return true for valid URL",
			server:   sampleFQDNServerURL,
			expected: true,
		},
		{
			name:     "should return false for invalid scheme",
			server:   "http://api.my-cluster.domain.example.com:6443",
			expected: false,
		},
		{
			name:     "should return false for missing api prefix",
			server:   "https://my-cluster.domain.example.com:6443",
			expected: false,
		},
		{
			name:     "should return false for missing port",
			server:   "https://api.my-cluster.domain.example.com",
			expected: false,
		},
		{
			name:     "should return false for invalid URL",
			server:   "not-a-url",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ValidateServerUrlFormat(tc.server)
			if result != tc.expected {
				t.Errorf("expected %v but got %v", tc.expected, result)
			}
		})
	}
}

func TestExtractClusterName(t *testing.T) {
	testCases := []struct {
		name     string
		server   string
		expected string
	}{
		{
			name:     "should extract cluster name from valid URL",
			server:   sampleFQDNServerURL,
			expected: sampleClusterName,
		},
		{
			name:     "should return input for invalid URL",
			server:   "invalid-server",
			expected: "invalid-server",
		},
		{
			name:     "should return input for in-cluster value",
			server:   inClusterAlias,
			expected: inClusterAlias,
		},
		{
			name:     "should return input for kubernetes.svc.cluster.local",
			server:   inClusterServerURL,
			expected: inClusterAlias,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractClusterName(tc.server)
			if result != tc.expected {
				t.Errorf("expected %v but got %v", tc.expected, result)
			}
		})
	}
}

func TestBuildServerUrl(t *testing.T) {
	testCases := []struct {
		name        string
		clusterName string
		domain      string
		expected    string
	}{
		{
			name:        "should build server URL correctly",
			clusterName: sampleClusterName,
			domain:      "example.com",
			expected:    sampleClusterServerURL,
		},
		{
			name:        "should build server URL with different domain",
			clusterName: "test-cluster",
			domain:      "test.io",
			expected:    "https://api.test-cluster.test.io:6443",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := BuildServerUrl(tc.clusterName, tc.domain)
			if result != tc.expected {
				t.Errorf("expected %v but got %v", tc.expected, result)
			}
		})
	}
}

func TestGetCurrentNamespace(t *testing.T) {
	testCases := []struct {
		name        string
		fileContent string
		createFile  bool
		expectError bool
		expectedNs  string
	}{
		{
			name:        "should read namespace from file",
			fileContent: "test-namespace\n",
			createFile:  true,
			expectError: false,
			expectedNs:  sampleNamespaceName,
		},
		{
			name:        "should trim whitespace",
			fileContent: "  test-namespace  \n",
			createFile:  true,
			expectError: false,
			expectedNs:  sampleNamespaceName,
		},
		{
			name:        "should return error when file doesn't exist",
			createFile:  false,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			originalPath := common.WebhookNamespacePath
			tmpFile := "/tmp/test-namespace-" + tc.name
			common.WebhookNamespacePath = tmpFile
			defer func() {
				common.WebhookNamespacePath = originalPath
				_ = os.Remove(tmpFile)
			}()

			if tc.createFile {
				if err := os.WriteFile(tmpFile, []byte(tc.fileContent), 0644); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
			}

			result, err := GetCurrentNamespace()
			if tc.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tc.expectError && result != tc.expectedNs {
				t.Errorf("expected %v but got %v", tc.expectedNs, result)
			}
		})
	}
}

func TestIsManagementApplication(t *testing.T) {
	testCases := []struct {
		name             string
		argoInstanceName string
		applicationName  string
		expected         bool
	}{
		{
			name:             "should return true for management application",
			argoInstanceName: sampleArgoInstanceName,
			applicationName:  "argo-instance-mgmt",
			expected:         true,
		},
		{
			name:             "should return false for non-management application",
			argoInstanceName: sampleArgoInstanceName,
			applicationName:  "my-app",
			expected:         false,
		},
		{
			name:             "should return false for empty instance name",
			argoInstanceName: "",
			applicationName:  "argo-instance-mgmt",
			expected:         false,
		},
		{
			name:             "should return false for empty application name",
			argoInstanceName: sampleArgoInstanceName,
			applicationName:  "",
			expected:         false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsManagementApplication(tc.argoInstanceName, tc.applicationName)
			if result != tc.expected {
				t.Errorf("expected %v but got %v", tc.expected, result)
			}
		})
	}
}

func TestIsInCluster(t *testing.T) {
	testCases := []struct {
		name      string
		serverUrl string
		expected  bool
	}{
		{
			name:      "should return true for in-cluster",
			serverUrl: inClusterAlias,
			expected:  true,
		},
		{
			name:      "should return true for kubernetes.svc.cluster.local",
			serverUrl: inClusterServerURL,
			expected:  true,
		},
		{
			name:      "should return false for external cluster",
			serverUrl: sampleClusterServerURL,
			expected:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsInCluster(tc.serverUrl)
			if result != tc.expected {
				t.Errorf("expected %v but got %v", tc.expected, result)
			}
		})
	}
}

func TestBypassLabelExists(t *testing.T) {
	testCases := []struct {
		name        string
		namespace   *corev1.Namespace
		clusterName string
		expected    bool
		expectError bool
	}{
		{
			name: "should return true for global bypass label",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: sampleNamespaceObjectName,
					Labels: map[string]string{
						common.AdminBypassLabel: common.LabelValueTrue,
					},
				},
			},
			clusterName: sampleClusterName,
			expected:    true,
			expectError: false,
		},
		{
			name: "should return true for cluster-specific bypass label",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: sampleNamespaceObjectName,
					Labels: map[string]string{
						common.AdminBypassLabel + "-my-cluster": common.LabelValueTrue,
					},
				},
			},
			clusterName: sampleClusterName,
			expected:    true,
			expectError: false,
		},
		{
			name: "should return false for different cluster label",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: sampleNamespaceObjectName,
					Labels: map[string]string{
						common.AdminBypassLabel + "-other-cluster": common.LabelValueTrue,
					},
				},
			},
			clusterName: sampleClusterName,
			expected:    false,
			expectError: false,
		},
		{
			name: "should return false when label value is not true",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: sampleNamespaceObjectName,
					Labels: map[string]string{
						common.AdminBypassLabel: "false",
					},
				},
			},
			clusterName: sampleClusterName,
			expected:    false,
			expectError: false,
		},
		{
			name: "should return false when no labels present",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   sampleNamespaceObjectName,
					Labels: map[string]string{},
				},
			},
			clusterName: sampleClusterName,
			expected:    false,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			cl := testutils.NewFakeClient(tc.namespace)

			ctx := context.Background()
			result, err := BypassLabelExists(ctx, cl, tc.namespace.Name, tc.clusterName)

			if tc.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tc.expected {
				t.Errorf("expected %v but got %v", tc.expected, result)
			}
		})
	}
}

func TestFormatFileSafeServerURL(t *testing.T) {
	testCases := []struct {
		name      string
		serverURL string
		expected  string
	}{
		{
			name:      "should remove https protocol and replace special chars",
			serverURL: sampleFQDNServerURL,
			expected:  "my-cluster-domain-example-com-6443",
		},
		{
			name:      "should remove http protocol",
			serverURL: "http://api.test-cluster.example.com:8080",
			expected:  "test-cluster-example-com-8080",
		},
		{
			name:      "should handle URL without protocol",
			serverURL: "api.cluster.example.com:6443",
			expected:  "cluster-example-com-6443",
		},
		{
			name:      "should handle simple cluster name",
			serverURL: sampleClusterName,
			expected:  sampleClusterName,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := FormatFileSafeServerURL(tc.serverURL)
			if result != tc.expected {
				t.Errorf("expected %v but got %v", tc.expected, result)
			}
		})
	}
}

func TestGetNamespacesList(t *testing.T) {
	testCases := []struct {
		name     string
		secret   *corev1.Secret
		expected []string
	}{
		{
			name: "should parse comma-separated namespaces",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					common.NamespaceKey: []byte("ns1,ns2,ns3"),
				},
			},
			expected: []string{"ns1", "ns2", "ns3"},
		},
		{
			name: "should return single namespace",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					common.NamespaceKey: []byte("ns1"),
				},
			},
			expected: []string{"ns1"},
		},
		{
			name: "should return empty slice for empty string",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					common.NamespaceKey: []byte(""),
				},
			},
			expected: []string{},
		},
		{
			name: "should return empty slice when key missing",
			secret: &corev1.Secret{
				Data: map[string][]byte{},
			},
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractNamespacesFromSecret(tc.secret)
			if len(result) != len(tc.expected) {
				t.Errorf("expected length %d but got %d", len(tc.expected), len(result))
			}
			for i, ns := range tc.expected {
				if result[i] != ns {
					t.Errorf("expected namespace at index %d to be %s but got %s", i, ns, result[i])
				}
			}
		})
	}
}

func TestFetchArgoInstanceName(t *testing.T) {
	testCases := []struct {
		name        string
		configMap   *corev1.ConfigMap
		expectError bool
		expected    string
	}{
		{
			name: "should fetch instance name successfully",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoInstanceConfigMapName,
					Namespace: sampleNamespaceName,
				},
				Data: map[string]string{
					common.ArgoInstanceNameConfigMapKey: "my-argo-instance",
				},
			},
			expectError: false,
			expected:    "my-argo-instance",
		},
		{
			name: "should return error when key missing",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoInstanceConfigMapName,
					Namespace: sampleNamespaceName,
				},
				Data: map[string]string{},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			cl := testutils.NewFakeClient(tc.configMap)

			ctx := context.Background()
			result, err := FetchArgoInstanceName(ctx, cl, tc.configMap.Namespace)

			if tc.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tc.expectError && result != tc.expected {
				t.Errorf("expected %v but got %v", tc.expected, result)
			}
		})
	}
}

func TestFetchArgoInstanceUsers(t *testing.T) {
	testCases := []struct {
		name        string
		configMap   *corev1.ConfigMap
		expectError bool
		expected    []string
	}{
		{
			name: "should fetch users successfully",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoInstanceConfigMapName,
					Namespace: sampleNamespaceName,
				},
				Data: map[string]string{
					common.ArgoInstanceUsersConfigMapKey: "user1,user2,user3",
				},
			},
			expectError: false,
			expected:    []string{sampleUser, "user2", "user3"},
		},
		{
			name: "should fetch single user",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoInstanceConfigMapName,
					Namespace: sampleNamespaceName,
				},
				Data: map[string]string{
					common.ArgoInstanceUsersConfigMapKey: sampleUser,
				},
			},
			expectError: false,
			expected:    []string{sampleUser},
		},
		{
			name: "should return error when key missing",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoInstanceConfigMapName,
					Namespace: sampleNamespaceName,
				},
				Data: map[string]string{},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			cl := testutils.NewFakeClient(tc.configMap)

			ctx := context.Background()
			result, err := FetchArgoInstanceUsers(ctx, cl, tc.configMap.Namespace)

			if tc.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tc.expectError {
				if len(result) != len(tc.expected) {
					t.Errorf("expected length %d but got %d", len(tc.expected), len(result))
				}
				for i, user := range tc.expected {
					if result[i] != user {
						t.Errorf("expected user at index %d to be %s but got %s", i, user, result[i])
					}
				}
			}
		})
	}
}

func TestFetchClusterToken(t *testing.T) {
	testCases := []struct {
		name        string
		configMap   *corev1.ConfigMap
		serverURL   string
		expectError bool
		expected    string
	}{
		{
			name: "should fetch token successfully",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ClusterTokensConfigMapName,
					Namespace: sampleNamespaceName,
				},
				Data: map[string]string{
					"my-cluster-example-com-6443-token": "test-token",
				},
			},
			serverURL:   sampleClusterServerURL,
			expectError: false,
			expected:    "test-token",
		},
		{
			name: "should return error when token key missing",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ClusterTokensConfigMapName,
					Namespace: sampleNamespaceName,
				},
				Data: map[string]string{},
			},
			serverURL:   sampleClusterServerURL,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			cl := testutils.NewFakeClient(tc.configMap)

			ctx := context.Background()
			result, err := FetchClusterToken(ctx, cl, tc.configMap.Namespace, tc.serverURL)

			if tc.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tc.expectError && result != tc.expected {
				t.Errorf("expected %v but got %v", tc.expected, result)
			}
		})
	}
}
func TestResolveDestinationServer(t *testing.T) {
	testCases := []struct {
		name          string
		app           *argoprojv1alpha1.Application
		existingObjs  []client.Object
		expected      string
		expectError   bool
		errorContains string
	}{
		{
			name: "should return server if set",
			app: &argoprojv1alpha1.Application{
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server: "https://my-server.com",
					},
				},
			},
			expected:    "https://my-server.com",
			expectError: false,
		},
		{
			name: "should return in-cluster server if name is in-cluster",
			app: &argoprojv1alpha1.Application{
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Name: inClusterAlias,
					},
				},
			},
			expected:    inClusterServerURL,
			expectError: false,
		},
		{
			name: "should return server from secret if name is set",
			app: &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: sampleArgoCDNamespace,
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Name: sampleClusterName,
					},
				},
			},
			existingObjs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-cluster-secret",
						Namespace: sampleArgoCDNamespace,
						Labels: map[string]string{
							common.ArgoCDSecretTypeLabelKey: common.ArgoCDSecretTypeClusterValue,
						},
					},
					Data: map[string][]byte{
						"name":   []byte(sampleClusterName),
						"server": []byte("https://my-cluster-server.com"),
					},
				},
			},
			expected:    "https://my-cluster-server.com",
			expectError: false,
		},
		{
			name: "should fail if secret not found",
			app: &argoprojv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: sampleArgoCDNamespace,
				},
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Name: "unknown-cluster",
					},
				},
			},
			expectError:   true,
			errorContains: "not found",
		},
		{
			name: "should fail if neither server nor name set",
			app: &argoprojv1alpha1.Application{
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{},
				},
			},
			expectError:   true,
			errorContains: "must be specified",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			c := testutils.NewFakeClient(tc.existingObjs...)
			ctx := context.Background()

			result, err := ResolveDestinationServer(ctx, c, tc.app)

			if tc.expectError {
				if err == nil {
					t.Error("expected error but got none")
				} else if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("expected error containing %q but got %q", tc.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tc.name == "should return in-cluster server if name is in-cluster" {
					if !strings.Contains(result, "kubernetes") && !strings.Contains(result, "svc") {
						t.Errorf("expected result to look like in-cluster address but got %q", result)
					}
				} else if result != tc.expected {
					t.Errorf("expected %q but got %q", tc.expected, result)
				}
			}
		})
	}
}
