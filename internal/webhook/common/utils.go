package common

import (
	"context"
	"fmt"
	"os"
	"strings"

	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetCurrentNamespace returns the current pod's namespace by reading the in-cluster service account namespace file.
func GetCurrentNamespace() (string, error) {
	data, err := os.ReadFile(WebhookNamespacePath)
	if err != nil {
		return "", fmt.Errorf("failed to read webhook's current namespace: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// IsManagementApplication checks whether the given application name
// follows the pattern "<argoInstanceName>-mgmt".
func IsManagementApplication(argoInstanceName, applicationName string) bool {
	if argoInstanceName == "" || applicationName == "" {
		return false
	}

	return applicationName == argoInstanceName+"-mgmt"
}

// BypassLabelExists returns a bool indicating whether an application-rbac-validator bypass label exists on the
// given namespace.
func BypassLabelExists(ctx context.Context,
	client client.Client,
	namespace string) (bool, error) {
	ns := &corev1.Namespace{}
	err := client.Get(ctx, types.NamespacedName{Name: namespace}, ns)
	if err != nil {
		return false, fmt.Errorf("client failed to get Namespace %s: %w", namespace, err)
	}

	value, ok := ns.Labels[AdminBypassLabel]

	return ok && value == "true", nil
}

// IsInCluster checks if the given serverUrl equals any known in-cluster values (e.g., "in-cluster", "kubernetes.svc.cluster.local").
func IsInCluster(serverUrl string) bool {
	for _, val := range InClusterValues {
		if serverUrl == val {
			return true
		}
	}

	return false
}

// fetchConfigMapValue retrieves a specific key's value from a ConfigMap in the given namespace.
func fetchConfigMapValue(
	ctx context.Context,
	k8sClient client.Client,
	namespace string,
	configMapName string,
	key string,
) (string, error) {
	var cm corev1.ConfigMap
	if err := k8sClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      configMapName,
	}, &cm); err != nil {
		return "", fmt.Errorf("failed to get ConfigMap %q: %w", configMapName, err)
	}

	val, ok := cm.Data[key]
	if !ok {
		return "", fmt.Errorf("key %q not found in ConfigMap %q", key, configMapName)
	}

	return strings.TrimSpace(val), nil
}

// FetchArgoInstanceName extracts the Application's argocd instance name from the argo-config ConfigMap inside the Application namespace.
func FetchArgoInstanceName(ctx context.Context, k8sClient client.Client, appNamespace string) (string, error) {
	value, err := fetchConfigMapValue(ctx, k8sClient, appNamespace, ArgoInstanceConfigMapName, ArgoInstanceNameConfigMapKey)
	if err != nil {
		return "", err
	}

	return value, nil
}

// FetchArgoInstanceUsers extracts the Application's admins from the argo-config ConfigMap inside the Application namespace.
func FetchArgoInstanceUsers(ctx context.Context, k8sClient client.Client, appNamespace string) ([]string, error) {
	value, err := fetchConfigMapValue(ctx, k8sClient, appNamespace, ArgoInstanceConfigMapName, ArgoInstanceUsersConfigMapKey)
	if err != nil {
		return nil, err
	}

	return strings.Split(value, "\n"), nil
}

// FetchClusterToken fetches the token for the destination cluster.
func FetchClusterToken(ctx context.Context, k8sClient client.Client, appNamespace string, serverURL string) (
	string, error) {
	configMapKey := FormatServerURL(serverURL) + "-token"

	value, err := fetchConfigMapValue(ctx, k8sClient, appNamespace, ClusterTokensConfigMapName, configMapKey)
	if err != nil {
		return "", err
	}

	return value, nil
}

// FormatServerURL converts a server URL string into a file-safe name by removing protocols and replacing
// special characters (e.g., ".", ":", "/") with hyphens ("-").
func FormatServerURL(serverURL string) string {
	protocolPrefixes := []string{"https://", "http://"}
	var safeName strings.Builder

	for _, prefix := range protocolPrefixes {
		serverURL = strings.TrimPrefix(serverURL, prefix)
	}

	for _, ch := range serverURL {
		switch ch {
		case '.', ':', '/':
			safeName.WriteRune('-')
		default:
			safeName.WriteRune(ch)
		}
	}

	return safeName.String()
}

// BuildClusterClient creates a kubernetes client for the destination cluster.
func BuildClusterClient(serverURL, token string) (kubernetes.Interface, error) {
	config := &rest.Config{
		Host:        serverURL,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
	return kubernetes.NewForConfig(config)
}

// isNamespaceAdmin checks if the user has admin access to a namespace.
func isNamespaceAdmin(ctx context.Context, client kubernetes.Interface, user, namespace string) (bool, error) {
	res, err := client.AuthorizationV1().SubjectAccessReviews().Create(ctx, buildSubjectAccessReview(user, namespace), metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("SubjectAccessReview failed: %w", err)
	}

	return res.Status.Allowed, nil
}

// buildSubjectAccessReview creates a SubjectAccessReview.
func buildSubjectAccessReview(user, namespace string) *authv1.SubjectAccessReview {
	return &authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			User: user,
			ResourceAttributes: &authv1.ResourceAttributes{
				Namespace: namespace,
				Verb:      AdminAccessLevel,
				Resource:  AdminAccessLevel,
			},
		},
	}
}

// EnsureAnyAdminHasNamespaceAccess verifies that at least one admin has admin-level access to the given namespace in
// the given cluster.
func EnsureAnyAdminHasNamespaceAccess(
	ctx context.Context,
	client kubernetes.Interface,
	admins []string,
	namespace, cluster string,
) error {
	for _, admin := range admins {
		isAllowed, err := isNamespaceAdmin(ctx, client, admin, namespace)
		if err != nil {
			return fmt.Errorf("error checking access for user %s: %w", admin, err)
		}
		if isAllowed {
			return nil
		}
	}
	return fmt.Errorf("no users have admin access to namespace %s in cluster %s", namespace, cluster)
}
