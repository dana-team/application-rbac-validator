package utils

import (
	"time"
)

const (
	TestDestinationNamespace              = "test-namespace"
	DefaultServerPort                     = "6443"
	BypassDestinationNamespace            = "bypass-namespace"
	TestDestinationServerName             = "test-cluster"
	TestDomain                            = "example.com"
	ClusterHostname                       = TestDestinationServerName + "." + TestDomain
	TestDestinationServerUrl              = "https://api." + ClusterHostname + ":" + DefaultServerPort
	ErrorTokenServerUrl                   = "error-token-server"
	ArgoInstanceUsersConfigMapData        = "admin1,admin2,admin3"
	InvalidArgoInstanceUsersConfigMapData = "admin2,admin3,admin4"
	ArgoInstanceNameConfigMapData         = "argo-instance-name"
	InvalidArgoInstanceUsersConfigMapKey  = "not-users"
	InvalidArgoInstanceNameConfigMapKey   = "not-project-name"
	WebhookNamespaceDir                   = "/tmp"
	WebhookNamespaceTestPath              = WebhookNamespaceDir + "/namespace"
)

const (
	DefaultTimeout    = 60 * time.Second
	DefaultInterval   = 1 * time.Second
	DefaultEventually = 2 * time.Second
)
