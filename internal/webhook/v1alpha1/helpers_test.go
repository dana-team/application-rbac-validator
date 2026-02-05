package v1alpha1

import (
	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dana-team/application-rbac-validator/internal/common"
	testutils "github.com/dana-team/application-rbac-validator/test/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var testCases = []struct {
	name                           string
	spec                           argoprojv1alpha1.ApplicationSpec
	serverTokenKey                 string
	argoInstanceNameConfigMapKey   string
	isManagementApplication        bool
	argoInstanceUsersConfigMapKey  string
	argoInstanceUsersConfigMapData string
	bypassLabel                    string
	expectToSucceed                bool
	clusterSecret                  *corev1.Secret
}{
	{
		name: "should reject Application with empty spec",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{},
		},
		argoInstanceNameConfigMapKey:  testutils.InvalidArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: testutils.InvalidArgoInstanceUsersConfigMapKey,
	},
	{
		name: "should reject Application if the config map does not have the instance name matching key",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testutils.TestDestinationNamespace,
				Server:    testutils.TestDestinationServerName,
			},
		},
		argoInstanceNameConfigMapKey:  testutils.InvalidArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: testutils.InvalidArgoInstanceUsersConfigMapKey,
	},
	{
		name: "should reject Application if the destination server is the same as the Application's current cluster",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testutils.TestDestinationNamespace,
				Server:    common.InClusterValues[0],
			},
		},
		argoInstanceNameConfigMapKey:  common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: testutils.InvalidArgoInstanceUsersConfigMapKey,
	},
	{
		name: "should reject Application with token fetch error",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testutils.TestDestinationNamespace,
				Server:    testutils.TestDestinationServerName,
			},
		},
		serverTokenKey:                testutils.ErrorTokenServerUrl,
		argoInstanceNameConfigMapKey:  common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: testutils.InvalidArgoInstanceUsersConfigMapKey,
	},
	{
		name: "should reject Application if the config map does not have admins matching key",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testutils.TestDestinationNamespace,
				Server:    testutils.TestDestinationServerName,
			},
		},
		serverTokenKey:                 testutils.TestDestinationServerUrl,
		argoInstanceNameConfigMapKey:   common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey:  testutils.InvalidArgoInstanceUsersConfigMapKey,
		argoInstanceUsersConfigMapData: testutils.ArgoInstanceUsersConfigMapData,
	},
	{
		name: "should reject Application if the users in the config map does not have permissions",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testutils.TestDestinationNamespace,
				Server:    testutils.TestDestinationServerName,
			},
		},
		serverTokenKey:                 testutils.TestDestinationServerUrl,
		argoInstanceNameConfigMapKey:   common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey:  common.ArgoInstanceUsersConfigMapKey,
		argoInstanceUsersConfigMapData: testutils.InvalidArgoInstanceUsersConfigMapData,
	},
	{
		name: "should allow valid Application",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testutils.TestDestinationNamespace,
				Server:    testutils.TestDestinationServerName,
			},
		},
		serverTokenKey:                 testutils.TestDestinationServerUrl,
		argoInstanceNameConfigMapKey:   common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey:  common.ArgoInstanceUsersConfigMapKey,
		argoInstanceUsersConfigMapData: testutils.ArgoInstanceUsersConfigMapData,
		expectToSucceed:                true,
	},
	{
		name: "should allow valid Application with full destination server url",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testutils.TestDestinationNamespace,
				Server:    testutils.TestDestinationServerUrl,
			},
		},
		serverTokenKey:                 testutils.TestDestinationServerUrl,
		argoInstanceNameConfigMapKey:   common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey:  common.ArgoInstanceUsersConfigMapKey,
		argoInstanceUsersConfigMapData: testutils.ArgoInstanceUsersConfigMapData,
		expectToSucceed:                true,
	},
	{
		name: "should allow valid Application with general bypass label",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testutils.TestDestinationNamespace,
				Server:    testutils.TestDestinationServerName,
			},
		},
		argoInstanceNameConfigMapKey:  common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: testutils.InvalidArgoInstanceUsersConfigMapKey,
		bypassLabel:                   common.AdminBypassLabel,
		expectToSucceed:               true,
	},
	{
		name: "should allow valid Application with specific destination bypass label",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testutils.TestDestinationNamespace,
				Server:    testutils.TestDestinationServerName,
			},
		},
		argoInstanceNameConfigMapKey:  common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: testutils.InvalidArgoInstanceUsersConfigMapKey,
		bypassLabel:                   common.AdminBypassLabel + "-" + testutils.TestDestinationServerName,
		expectToSucceed:               true,
	},
	{
		name: "should reject valid Application with wrong destination bypass label",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testutils.TestDestinationNamespace,
				Server:    testutils.TestDestinationServerName,
			},
		},
		argoInstanceNameConfigMapKey:  common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: testutils.InvalidArgoInstanceUsersConfigMapKey,
		bypassLabel:                   common.AdminBypassLabel + "-not-my-cluster",
	},
	{
		name: "should allow valid Application with in-cluster bypass label",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testutils.TestDestinationNamespace,
				Server:    common.InClusterValues[0],
			},
		},
		argoInstanceNameConfigMapKey:  common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: testutils.InvalidArgoInstanceUsersConfigMapKey,
		bypassLabel:                   common.AdminBypassLabel + "-" + common.InClusterValues[0],
		expectToSucceed:               true,
	},
	{
		name: "should allow valid Application with in-cluster bypass label when destination is kubernetes.svc.cluster.local",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testutils.TestDestinationNamespace,
				Server:    common.InClusterValues[1],
			},
		},
		argoInstanceNameConfigMapKey:  common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: testutils.InvalidArgoInstanceUsersConfigMapKey,
		bypassLabel:                   common.AdminBypassLabel + "-" + common.InClusterValues[0],
		expectToSucceed:               true,
	},
	{
		name: "should allow valid management Application",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testutils.TestDestinationNamespace,
				Server:    testutils.TestDestinationServerName,
			},
		},
		argoInstanceNameConfigMapKey:  common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: testutils.InvalidArgoInstanceUsersConfigMapKey,
		isManagementApplication:       true,
		expectToSucceed:               true,
	},
	{
		name: "should resolve destination name to server from secret",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testutils.TestDestinationNamespace,
				Name:      "test-cluster",
			},
		},
		serverTokenKey:                 testutils.TestDestinationServerUrl,
		argoInstanceNameConfigMapKey:   common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey:  common.ArgoInstanceUsersConfigMapKey,
		argoInstanceUsersConfigMapData: testutils.ArgoInstanceUsersConfigMapData,
		expectToSucceed:                true,
		clusterSecret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster-secret",
				Namespace: "test-ns",
				Labels: map[string]string{
					common.ArgoCDSecretTypeLabelKey: common.ArgoCDSecretTypeClusterValue,
				},
			},
			Data: map[string][]byte{
				"name":   []byte("test-cluster"),
				"server": []byte(testutils.TestDestinationServerUrl),
			},
		},
	},
}
