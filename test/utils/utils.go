package utils

import (
	"math/rand"
	"strings"

	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dana-team/application-rbac-validator/internal/common"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crtfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// NewMockedDestinationClusterClient returns a fake Kubernetes client with custom reactors
func NewMockedDestinationClusterClient() *fake.Clientset {
	client := fake.NewClientset()

	client.Fake.PrependReactor("create", "subjectaccessreviews",
		func(action testing.Action) (bool, runtime.Object, error) {
			sar := action.(testing.CreateAction).GetObject().(*authv1.SubjectAccessReview)
			if sar.Spec.User == strings.Split(ArgoInstanceUsersConfigMapData, ",")[0] {
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

		if namespaceName == BypassDestinationNamespace {
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

// GenerateRandomSuffix generates a random string of the specified length
func GenerateRandomSuffix(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	var sb strings.Builder
	sb.Grow(n)
	for i := 0; i < n; i++ {
		sb.WriteByte(letters[rand.Intn(len(letters))])
	}
	return sb.String()
}

// GenerateTestApplication creates a test Argo CD Application object with random name.
func GenerateTestApplication(namespace, destinationServer, destinationNamespace string) *argoprojv1alpha1.Application {
	return &argoprojv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-application-" + GenerateRandomSuffix(5),
			Namespace: namespace,
		},
		Spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Server:    destinationServer,
				Namespace: destinationNamespace,
			},
		},
	}
}

// GenerateTestApplicationWithDestinationName creates a test Argo CD Application object
// with destination name instead of server.
func GenerateTestApplicationWithDestinationName(
	namespace,
	destinationName,
	destinationNamespace string,
) *argoprojv1alpha1.Application {
	return &argoprojv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-application-" + GenerateRandomSuffix(5),
			Namespace: namespace,
		},
		Spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Name:      destinationName,
				Namespace: destinationNamespace,
			},
		},
	}
}

// NewFakeClient creates a new fake client with the provided initial objects.
func NewFakeClient(initObjs ...client.Object) client.WithWatch {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = argoprojv1alpha1.AddToScheme(scheme)

	return crtfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(initObjs...).
		Build()

}
