package v1alpha1

import (
	"strings"

	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"

	"github.com/dana-team/application-rbac-validator/internal/webhook/common"
)

const (
	testDestinationNamespace              = "test-namespace"
	bypassDestinationNamespace            = "bypass-namespace"
	testDestinationServerUrl              = "https://api.my-cluster.domain.example.com:" + common.DefaultServerUrlPort
	testDestinationServerName             = "my-cluster"
	errorTokenServerUrl                   = "error-token-server"
	argoInstanceUsersConfigMapData        = "admin1,admin2,admin3"
	invalidArgoInstanceUsersConfigMapData = "admin2,admin3,admin4"
	ArgoInstanceNameConfigMapData         = "argo-instance-name"
	invalidArgoInstanceUsersConfigMapKey  = "not-users"
	invalidArgoInstanceNameConfigMapKey   = "not-project-name"
	webhookNamespaceDir                   = "/tmp"
	webhookNamespaceTestPath              = webhookNamespaceDir + "/namespace"
)

var (
	validator          ApplicationCustomValidator
	typeNamespacedName types.NamespacedName
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
}{
	{
		name: "should reject Application with empty spec",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{},
		},
		argoInstanceNameConfigMapKey:  invalidArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: invalidArgoInstanceUsersConfigMapKey,
	},
	{
		name: "should reject Application if the config map does not have the instance name matching key",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testDestinationNamespace,
				Server:    testDestinationServerName,
			},
		},
		argoInstanceNameConfigMapKey:  invalidArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: invalidArgoInstanceUsersConfigMapKey,
	},
	{
		name: "should reject Application if the destination server is the same as the Application's current cluster",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testDestinationNamespace,
				Server:    common.InClusterValues[0],
			},
		},
		argoInstanceNameConfigMapKey:  common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: invalidArgoInstanceUsersConfigMapKey,
	},
	{
		name: "should reject Application with token fetch error",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testDestinationNamespace,
				Server:    testDestinationServerName,
			},
		},
		serverTokenKey:                errorTokenServerUrl,
		argoInstanceNameConfigMapKey:  common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: invalidArgoInstanceUsersConfigMapKey,
	},
	{
		name: "should reject Application if the config map does not have admins matching key",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testDestinationNamespace,
				Server:    testDestinationServerName,
			},
		},
		serverTokenKey:                 testDestinationServerUrl,
		argoInstanceNameConfigMapKey:   common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey:  invalidArgoInstanceUsersConfigMapKey,
		argoInstanceUsersConfigMapData: argoInstanceUsersConfigMapData,
	},
	{
		name: "should reject Application if the users in the config map does not have permissions",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testDestinationNamespace,
				Server:    testDestinationServerName,
			},
		},
		serverTokenKey:                 testDestinationServerUrl,
		argoInstanceNameConfigMapKey:   common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey:  common.ArgoInstanceUsersConfigMapKey,
		argoInstanceUsersConfigMapData: invalidArgoInstanceUsersConfigMapData,
	},
	{
		name: "should allow valid Application",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testDestinationNamespace,
				Server:    testDestinationServerName,
			},
		},
		serverTokenKey:                 testDestinationServerUrl,
		argoInstanceNameConfigMapKey:   common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey:  common.ArgoInstanceUsersConfigMapKey,
		argoInstanceUsersConfigMapData: argoInstanceUsersConfigMapData,
		expectToSucceed:                true,
	},
	{
		name: "should allow valid Application with full destination server url",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testDestinationNamespace,
				Server:    testDestinationServerUrl,
			},
		},
		serverTokenKey:                 testDestinationServerUrl,
		argoInstanceNameConfigMapKey:   common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey:  common.ArgoInstanceUsersConfigMapKey,
		argoInstanceUsersConfigMapData: argoInstanceUsersConfigMapData,
		expectToSucceed:                true,
	},
	{
		name: "should allow valid Application with general bypass label",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testDestinationNamespace,
				Server:    testDestinationServerName,
			},
		},
		argoInstanceNameConfigMapKey:  common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: invalidArgoInstanceUsersConfigMapKey,
		bypassLabel:                   common.AdminBypassLabel,
		expectToSucceed:               true,
	},
	{
		name: "should allow valid Application with specific destination bypass label",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testDestinationNamespace,
				Server:    testDestinationServerName,
			},
		},
		argoInstanceNameConfigMapKey:  common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: invalidArgoInstanceUsersConfigMapKey,
		bypassLabel:                   common.AdminBypassLabel + "-" + testDestinationServerName,
		expectToSucceed:               true,
	},
	{
		name: "should reject valid Application with wrong destination bypass label",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testDestinationNamespace,
				Server:    testDestinationServerName,
			},
		},
		argoInstanceNameConfigMapKey:  common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: invalidArgoInstanceUsersConfigMapKey,
		bypassLabel:                   common.AdminBypassLabel + "-not-my-cluster",
	},
	{
		name: "should allow valid Application with in-cluster bypass label",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testDestinationNamespace,
				Server:    common.InClusterValues[0],
			},
		},
		argoInstanceNameConfigMapKey:  common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: invalidArgoInstanceUsersConfigMapKey,
		bypassLabel:                   common.AdminBypassLabel + "-" + common.InClusterValues[0],
		expectToSucceed:               true,
	},
	{
		name: "should allow valid Application with in-cluster bypass label when destination is kubernetes.svc.cluster.local",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testDestinationNamespace,
				Server:    common.InClusterValues[1],
			},
		},
		argoInstanceNameConfigMapKey:  common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: invalidArgoInstanceUsersConfigMapKey,
		bypassLabel:                   common.AdminBypassLabel + "-" + common.InClusterValues[0],
		expectToSucceed:               true,
	},
	{
		name: "should allow valid management Application",
		spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Namespace: testDestinationNamespace,
				Server:    testDestinationServerName,
			},
		},
		argoInstanceNameConfigMapKey:  common.ArgoInstanceNameConfigMapKey,
		argoInstanceUsersConfigMapKey: invalidArgoInstanceUsersConfigMapKey,
		isManagementApplication:       true,
		expectToSucceed:               true,
	},
}

// NewMockedDestinationClusterClient returns a fake Kubernetes client with custom reactors
func NewMockedDestinationClusterClient() *fake.Clientset {
	client := fake.NewClientset()

	client.Fake.PrependReactor("create", "subjectaccessreviews", func(action testing.Action) (bool, runtime.Object, error) {
		sar := action.(testing.CreateAction).GetObject().(*authv1.SubjectAccessReview)
		if sar.Spec.User == strings.Split(argoInstanceUsersConfigMapData, ",")[0] {
			return true, &authv1.SubjectAccessReview{
				Status: authv1.SubjectAccessReviewStatus{Allowed: true},
			}, nil
		}
		return true, &authv1.SubjectAccessReview{
			Status: authv1.SubjectAccessReviewStatus{Allowed: false},
		}, nil
	})

	client.Fake.PrependReactor("get", "namespaces", func(action testing.Action) (bool, runtime.Object, error) {
		getAction := action.(testing.GetAction)
		namespaceName := getAction.GetName()

		if namespaceName == bypassDestinationNamespace {
			return true, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
					Labels: map[string]string{
						common.AdminBypassLabel: "true",
					},
				},
			}, nil
		}

		return true, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   namespaceName,
				Labels: map[string]string{},
			},
		}, nil
	})

	return client
}
