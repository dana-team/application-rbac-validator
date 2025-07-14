package common

import "os"

const (
	ClusterTokensConfigMapName    = "application-rbac-validator-cluster-tokens"
	ArgoInstanceConfigMapName     = "argo-config"
	ArgoInstanceUsersConfigMapKey = "instance_users"
	ArgoInstanceNameConfigMapKey  = "instance_name"
	InstanceUsersAccessLevel      = "pods"
	AdminBypassLabel              = "argocd.dana.io/bypass-rbac-validation"
	DefaultServerUrlPort          = "6443"
)

var (
	InClusterValues      = []string{"in-cluster", "kubernetes.svc.cluster.local"}
	WebhookNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

var ServerUrlDomain = os.Getenv("KUBERNETES_CLUSTER_DOMAIN")
