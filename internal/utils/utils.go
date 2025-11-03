package utils

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"

	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dana-team/application-rbac-validator/internal/common"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IsNotSpecUpdate checks if the only difference between the old and new Application objects is in their Status subresource.
// It returns true if only the status has changed, false otherwise.
func IsNotSpecUpdate(oldApp, newApp *argoprojv1alpha1.Application) bool {
	return reflect.DeepEqual(oldApp.Spec, newApp.Spec)
}

// ValidateServerUrlFormat checks whether the given destServer is a full, valid server URL according to this format:
// https://api.my-cluster.domain.example.com:port.
func ValidateServerUrlFormat(destServer string) bool {
	parsedUrl, err := url.Parse(destServer)
	if err != nil || parsedUrl.Scheme != "https" {
		return false
	}

	if !strings.HasPrefix(parsedUrl.Host, "api.") {
		return false
	}

	hostParts := strings.Split(parsedUrl.Host, ":")
	if len(hostParts) != 2 {
		return false
	}

	return true
}

// ExtractClusterName parses the destServer URL and returns the cluster name (e.g. "my-cluster")
// if the URL is valid and matches the expected format.
func ExtractClusterName(destServer string) string {
	if ValidateServerUrlFormat(destServer) {
		parsedUrl, _ := url.Parse(destServer)
		host := parsedUrl.Hostname()
		parts := strings.Split(host, ".")
		return parts[1]
	}

	return destServer
}

// BuildServerUrl constructs a full server URL from a partial cluster name according to this format:
// https://api.my-cluster.domain.example.com:port.
func BuildServerUrl(clusterName string) string {
	return fmt.Sprintf("https://api.%s.%s:%s", clusterName, common.ServerUrlDomain, common.DefaultServerUrlPort)
}

// GetCurrentNamespace returns the current pod's namespace by reading the in-cluster service account namespace file.
func GetCurrentNamespace() (string, error) {
	data, err := os.ReadFile(common.WebhookNamespacePath)
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
	namespace, clusterName string) (bool, error) {
	ns := &corev1.Namespace{}
	err := client.Get(ctx, types.NamespacedName{Name: namespace}, ns)
	if err != nil {
		return false, fmt.Errorf("client failed to get Namespace %s: %w", namespace, err)
	}
	return isBypassLabelValid(ns.Labels, clusterName), nil
}

// isBypassLabelValid checks if the given labels contain a valid AdminBypassLabel for the specified clusterName.
func isBypassLabelValid(labels map[string]string, clusterName string) bool {
	for key, value := range labels {
		if strings.HasPrefix(key, common.AdminBypassLabel) && value == "true" {
			suffix := strings.TrimPrefix(key, common.AdminBypassLabel)

			switch suffix {
			case "":
				return true

			case "-" + clusterName:
				return true

			case "-" + common.InClusterValues[0]:
				for _, InClusterValue := range common.InClusterValues {
					if clusterName == InClusterValue {
						return true
					}
				}
			}
		}
	}

	return false
}

// IsInCluster checks if the given serverUrl equals any known in-cluster values (e.g., "in-cluster", "kubernetes.default.svc.cluster.local").
func IsInCluster(serverUrl string) bool {
	for _, val := range common.InClusterValues {
		if strings.Contains(serverUrl, val) {
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
	value, err := fetchConfigMapValue(ctx, k8sClient, appNamespace, common.ArgoInstanceConfigMapName, common.ArgoInstanceNameConfigMapKey)
	if err != nil {
		return "", err
	}

	return value, nil
}

// FetchArgoInstanceUsers extracts the Application's admins from the argo-config ConfigMap inside the Application namespace.
func FetchArgoInstanceUsers(ctx context.Context, k8sClient client.Client, appNamespace string) ([]string, error) {
	value, err := fetchConfigMapValue(ctx, k8sClient, appNamespace, common.ArgoInstanceConfigMapName, common.ArgoInstanceUsersConfigMapKey)
	if err != nil {
		return nil, err
	}

	return strings.Split(value, ","), nil
}

// FetchClusterToken fetches the token for the destination cluster.
func FetchClusterToken(ctx context.Context, k8sClient client.Client, appNamespace string, serverURL string) (
	string, error) {
	configMapKey := FormatFileSafeServerURL(serverURL) + "-token"

	value, err := fetchConfigMapValue(ctx, k8sClient, appNamespace, common.ClusterTokensConfigMapName, configMapKey)
	if err != nil {
		return "", err
	}

	return value, nil
}

// FormatFileSafeServerURL converts a server URL string into a file-safe name by removing protocols and replacing
// special characters (e.g., ".", ":", "/") with hyphens ("-").
func FormatFileSafeServerURL(serverURL string) string {
	protocolPrefixes := []string{"https://", "http://"}
	var safeName strings.Builder

	for _, prefix := range protocolPrefixes {
		serverURL = strings.TrimPrefix(serverURL, prefix)
	}

	serverURL = strings.TrimPrefix(serverURL, "api.")

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
	for _, verb := range common.InstanceUsersAccessLevelVerbs {
		res, err := client.AuthorizationV1().SubjectAccessReviews().Create(
			ctx,
			buildSubjectAccessReview(user, namespace, verb),
			metav1.CreateOptions{},
		)

		if err != nil {
			return false, fmt.Errorf("SubjectAccessReview failed: %w", err)
		}

		if !res.Status.Allowed {
			return false, nil
		}
	}

	return true, nil
}

// buildSubjectAccessReview creates a SubjectAccessReview.
func buildSubjectAccessReview(user, namespace, verb string) *authv1.SubjectAccessReview {
	return &authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			User: user,
			ResourceAttributes: &authv1.ResourceAttributes{
				Namespace: namespace,
				Verb:      verb,
				Resource:  common.InstanceUsersAccessLevelResource,
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

// FetchDestinationClusterSecret retrieves the secret associated with the destination cluster of the given Application.
func FetchDestinationClusterSecret(ctx context.Context, k8sClient client.Client, app *argoprojv1alpha1.Application) (*corev1.Secret, error) {
	destinationURl, err := url.Parse(app.Spec.Destination.Server)
	if err != nil {
		return nil, fmt.Errorf("failed to parse destination server URL %s: %w", app.Spec.Destination.Server,
			err)
	}
	destination := strings.TrimPrefix(destinationURl.Hostname(), "api.")
	secretName := fmt.Sprintf("%s-%s", destination, common.SecretNameSuffix)
	secret := &corev1.Secret{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: app.Namespace}, secret)
	return secret, err
}

// ExtractNamespacesFromSecret extracts the list of namespaces from the cluster secret's data.
func ExtractNamespacesFromSecret(secret *corev1.Secret) []string {
	namespacesRaw, ok := secret.Data[common.NamespaceKey]
	if !ok || len(namespacesRaw) == 0 {
		return []string{}
	}
	return strings.Split(string(namespacesRaw), ",")

}

// FetchClusterDomain retrieves the cluster domain from the environment variable.
func FetchClusterDomain() {
	domain, found := os.LookupEnv(common.ClusterDomainEnvVar)
	if found {
		common.ServerUrlDomain = domain
		common.DomainEnvVarFound = true
	}
}

// IsDestinationNamespaceInUse checks if any other application is deploying to the same namespace in the same cluster.
func IsDestinationNamespaceInUse(applicationList *argoprojv1alpha1.ApplicationList, app *argoprojv1alpha1.Application, destinationNS string) bool {
	for _, otherApp := range applicationList.Items {
		if otherApp.Name != app.Name && otherApp.Spec.Destination.Namespace == destinationNS && otherApp.Spec.Destination.Server == app.Spec.Destination.Server {
			return true
		}
	}
	return false
}

// RetryUpdateSecret retries updating the secret with the new namespace list in case of a conflict.
func RetryUpdateSecret(ctx context.Context, k8sClient client.Client, app *argoprojv1alpha1.Application, namespaceList []string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		secret, err := FetchDestinationClusterSecret(ctx, k8sClient, app)
		if err != nil {
			return err
		}
		secret.Data[common.NamespaceKey] = []byte(strings.Join(namespaceList, ","))
		if err = k8sClient.Update(ctx, secret); err != nil {
			return err
		}

		return nil
	})
}

// ShouldBypassOptimization checks if the secret has the bypass optimization key set to "true".
func ShouldBypassOptimization(secret *corev1.Secret) bool {
	if secret.Labels == nil {
		return false
	}
	bypassValue, ok := secret.Labels[common.BypassOptimizationLabel]
	if !ok {
		return false
	}

	return strings.ToLower(bypassValue) == "true"
}
