package common

const (
	ClusterTokensConfigMapName       = "application-rbac-validator-cluster-tokens"
	ArgoInstanceConfigMapName        = "argo-config"
	ArgoInstanceUsersConfigMapKey    = "instance_users"
	ArgoInstanceNameConfigMapKey     = "instance_name"
	InstanceUsersAccessLevelResource = "pods"
	AdminBypassLabel                 = "argocd.dana.io/bypass-rbac-validation"
	BypassOptimizationLabel          = "argocd.dana.io/bypass-optimization"
	DefaultServerUrlPort             = "6443"
	NamespaceKey                     = "namespaces"
	ClusterResourcesKey              = "clusterResources"
)

var (
	InClusterValues               = []string{"in-cluster", "kubernetes.default.svc"}
	InstanceUsersAccessLevelVerbs = []string{"get", "list", "watch", "create", "update", "patch", "delete"}
	WebhookNamespacePath          = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	ClusterDomainEnvVar           = "KUBERNETES_CLUSTER_DOMAIN"
	ServerUrlDomain               = ""
	SecretNameSuffix              = "cluster-secret"
	DomainEnvVarFound             = false
)
