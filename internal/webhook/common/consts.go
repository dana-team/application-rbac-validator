package common

const (
	ClusterTokensConfigMapName    = "application-rbac-validator-cluster-tokens"
	ArgoInstanceConfigMapName     = "argo-config"
	ArgoInstanceUsersConfigMapKey = "instance_users"
	ArgoInstanceNameConfigMapKey  = "instance_name"
	AdminAccessLevel              = "*"
	AdminBypassLabel              = "argocd.dana.io/bypass-rbac-validation"
)

var (
	InClusterValues      = []string{"in-cluster", "kubernetes.svc.cluster.local"}
	WebhookNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)
