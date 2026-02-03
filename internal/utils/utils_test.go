package utils

import (
	"context"
	"os"
	"testing"

	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dana-team/application-rbac-validator/internal/common"
	testutils "github.com/dana-team/application-rbac-validator/test/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
						Server:    "test-server",
						Namespace: "test-namespace",
					},
				},
			},
			newApp: &argoprojv1alpha1.Application{
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    "test-server",
						Namespace: "test-namespace",
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
						Server:    "test-server",
						Namespace: "test-namespace",
					},
				},
			},
			newApp: &argoprojv1alpha1.Application{
				Spec: argoprojv1alpha1.ApplicationSpec{
					Destination: argoprojv1alpha1.ApplicationDestination{
						Server:    "test-server",
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
			server:   "https://api.my-cluster.domain.example.com:6443",
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
			server:   "https://api.my-cluster.domain.example.com:6443",
			expected: "my-cluster",
		},
		{
			name:     "should return input for invalid URL",
			server:   "invalid-server",
			expected: "invalid-server",
		},
		{
			name:     "should return input for in-cluster value",
			server:   "in-cluster",
			expected: "in-cluster",
		},
		{
			name:     "should return input for kubernetes.svc.cluster.local",
			server:   "kubernetes.svc.cluster.local",
			expected: "in-cluster",
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
			clusterName: "my-cluster",
			domain:      "example.com",
			expected:    "https://api.my-cluster.example.com:6443",
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
			originalDomain := common.ServerUrlDomain
			common.ServerUrlDomain = tc.domain
			defer func() { common.ServerUrlDomain = originalDomain }()

			result := BuildServerUrl(tc.clusterName)
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
			expectedNs:  "test-namespace",
		},
		{
			name:        "should trim whitespace",
			fileContent: "  test-namespace  \n",
			createFile:  true,
			expectError: false,
			expectedNs:  "test-namespace",
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
			argoInstanceName: "argo-instance",
			applicationName:  "argo-instance-mgmt",
			expected:         true,
		},
		{
			name:             "should return false for non-management application",
			argoInstanceName: "argo-instance",
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
			argoInstanceName: "argo-instance",
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
			serverUrl: "in-cluster",
			expected:  true,
		},
		{
			name:      "should return true for kubernetes.svc.cluster.local",
			serverUrl: "kubernetes.svc.cluster.local",
			expected:  true,
		},
		{
			name:      "should return false for external cluster",
			serverUrl: "https://api.my-cluster.example.com:6443",
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
					Name: "test-ns",
					Labels: map[string]string{
						common.AdminBypassLabel: "true",
					},
				},
			},
			clusterName: "my-cluster",
			expected:    true,
			expectError: false,
		},
		{
			name: "should return true for cluster-specific bypass label",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ns",
					Labels: map[string]string{
						common.AdminBypassLabel + "-my-cluster": "true",
					},
				},
			},
			clusterName: "my-cluster",
			expected:    true,
			expectError: false,
		},
		{
			name: "should return false for different cluster label",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ns",
					Labels: map[string]string{
						common.AdminBypassLabel + "-other-cluster": "true",
					},
				},
			},
			clusterName: "my-cluster",
			expected:    false,
			expectError: false,
		},
		{
			name: "should return false when label value is not true",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ns",
					Labels: map[string]string{
						common.AdminBypassLabel: "false",
					},
				},
			},
			clusterName: "my-cluster",
			expected:    false,
			expectError: false,
		},
		{
			name: "should return false when no labels present",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-ns",
					Labels: map[string]string{},
				},
			},
			clusterName: "my-cluster",
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
			serverURL: "https://api.my-cluster.domain.example.com:6443",
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
			serverURL: "my-cluster",
			expected:  "my-cluster",
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
					Namespace: "test-namespace",
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
					Namespace: "test-namespace",
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
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					common.ArgoInstanceUsersConfigMapKey: "user1,user2,user3",
				},
			},
			expectError: false,
			expected:    []string{"user1", "user2", "user3"},
		},
		{
			name: "should fetch single user",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoInstanceConfigMapName,
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					common.ArgoInstanceUsersConfigMapKey: "user1",
				},
			},
			expectError: false,
			expected:    []string{"user1"},
		},
		{
			name: "should return error when key missing",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoInstanceConfigMapName,
					Namespace: "test-namespace",
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
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"my-cluster-example-com-6443-token": "test-token",
				},
			},
			serverURL:   "https://api.my-cluster.example.com:6443",
			expectError: false,
			expected:    "test-token",
		},
		{
			name: "should return error when token key missing",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ClusterTokensConfigMapName,
					Namespace: "test-namespace",
				},
				Data: map[string]string{},
			},
			serverURL:   "https://api.my-cluster.example.com:6443",
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
