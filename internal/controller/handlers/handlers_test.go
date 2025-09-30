package handlers

import (
	"context"
	"testing"

	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dana-team/application-rbac-validator/internal/common"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func setupTest(t *testing.T) (client.Client, *runtime.Scheme) {
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = argoprojv1alpha1.AddToScheme(s)
	_ = corev1.AddToScheme(s)

	cl := fake.NewClientBuilder().WithScheme(s).Build()
	return cl, s
}

func TestHandleCreateOrUpdate_InCluster(t *testing.T) {
	cl, _ := setupTest(t)
	ctx := context.Background()
	log := logr.Discard()

	app := &argoprojv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "test-ns",
		},
		Spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Server:    "in-cluster",
				Namespace: "test-dest-ns",
			},
		},
	}

	err := HandleCreateOrUpdate(log, ctx, cl, app)
	if err != nil {
		t.Errorf("Expected no error for in-cluster destination, got %v", err)
	}

	// Verify no finalizer was added
	if controllerutil.ContainsFinalizer(app, common.FinalizerName) {
		t.Error("Expected no finalizer for in-cluster destination")
	}
}

func TestHandleCreateOrUpdate_ClusterwideAccess(t *testing.T) {
	cl, _ := setupTest(t)
	ctx := context.Background()
	log := logr.Discard()
	common.ServerUrlDomain = "example.com"

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster.example.com-cluster-tokentoken",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			common.NamespaceKey:        []byte("existing-ns"),
			common.ClusterResourcesKey: []byte("true"),
		},
	}
	if err := cl.Create(ctx, secret); err != nil {
		t.Fatalf("Failed to create secret: %v", err)
	}

	app := &argoprojv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "test-ns",
		},
		Spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Server:    "test-cluster",
				Namespace: "test-dest-ns",
			},
		},
	}

	err := HandleCreateOrUpdate(log, ctx, cl, app)
	if err != nil {
		t.Errorf("Expected no error for clusterwide destination, got %v", err)
	}

	// Verify secret was not modified
	updatedSecret := &corev1.Secret{}
	err = cl.Get(ctx, client.ObjectKeyFromObject(secret), updatedSecret)
	if err != nil {
		t.Fatalf("Failed to get secret: %v", err)
	}

	if string(updatedSecret.Data[common.NamespaceKey]) != "existing-ns" {
		t.Errorf("Expected secret to not be modified, got %s", string(updatedSecret.Data[common.NamespaceKey]))
	}
}

func TestHandleCreateOrUpdate_AddNamespaceToSecret(t *testing.T) {
	cl, _ := setupTest(t)
	ctx := context.Background()
	log := logr.Discard()
	common.ServerUrlDomain = "example.com"

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster.example.com-cluster-tokentoken",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			common.NamespaceKey: []byte("existing-ns"),
		},
	}
	if err := cl.Create(ctx, secret); err != nil {
		t.Fatalf("Failed to create secret: %v", err)
	}

	app := &argoprojv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "test-ns",
		},
		Spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Server:    "test-cluster",
				Namespace: "new-ns",
			},
		},
	}
	if err := cl.Create(ctx, app); err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	err := HandleCreateOrUpdate(log, ctx, cl, app)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify namespace was added to secret
	updatedSecret := &corev1.Secret{}
	err = cl.Get(ctx, client.ObjectKeyFromObject(secret), updatedSecret)
	if err != nil {
		t.Fatalf("Failed to get secret: %v", err)
	}

	namespaces := string(updatedSecret.Data[common.NamespaceKey])
	if namespaces != "existing-ns,new-ns" {
		t.Errorf("Expected namespaces to be 'existing-ns,new-ns', got %s", namespaces)
	}

	// Verify finalizer was added
	updatedApp := &argoprojv1alpha1.Application{}
	err = cl.Get(ctx, client.ObjectKeyFromObject(app), updatedApp)
	if err != nil {
		t.Fatalf("Failed to get app: %v", err)
	}
	if !controllerutil.ContainsFinalizer(updatedApp, common.FinalizerName) {
		t.Error("Expected finalizer to be added")
	}
}

func TestHandleCreateOrUpdate_NamespaceAlreadyExists(t *testing.T) {
	cl, _ := setupTest(t)
	ctx := context.Background()
	log := logr.Discard()
	common.ServerUrlDomain = "example.com"

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster.example.com-cluster-tokentoken",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			common.NamespaceKey: []byte("existing-ns,new-ns"),
		},
	}
	if err := cl.Create(ctx, secret); err != nil {
		t.Fatalf("Failed to create secret: %v", err)
	}

	app := &argoprojv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "test-ns",
		},
		Spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Server:    "test-cluster",
				Namespace: "new-ns",
			},
		},
	}
	if err := cl.Create(ctx, app); err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	err := HandleCreateOrUpdate(log, ctx, cl, app)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify namespace list didn't change
	updatedSecret := &corev1.Secret{}
	err = cl.Get(ctx, client.ObjectKeyFromObject(secret), updatedSecret)
	if err != nil {
		t.Fatalf("Failed to get secret: %v", err)
	}

	namespaces := string(updatedSecret.Data[common.NamespaceKey])
	if namespaces != "existing-ns,new-ns" {
		t.Errorf("Expected namespaces to remain 'existing-ns,new-ns', got %s", namespaces)
	}
}

func TestHandleDelete_RemoveNamespace(t *testing.T) {
	cl, _ := setupTest(t)
	ctx := context.Background()
	log := logr.Discard()
	common.ServerUrlDomain = "example.com"

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster.example.com-cluster-tokentoken",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			common.NamespaceKey: []byte("other-ns,test-dest-ns"),
		},
	}
	if err := cl.Create(ctx, secret); err != nil {
		t.Fatalf("Failed to create secret: %v", err)
	}

	app := &argoprojv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-app",
			Namespace:  "test-ns",
			Finalizers: []string{common.FinalizerName},
		},
		Spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Server:    "test-cluster",
				Namespace: "test-dest-ns",
			},
		},
	}
	if err := cl.Create(ctx, app); err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	err := HandleDelete(log, ctx, cl, app)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify namespace was removed from secret
	updatedSecret := &corev1.Secret{}
	err = cl.Get(ctx, client.ObjectKeyFromObject(secret), updatedSecret)
	if err != nil {
		t.Fatalf("Failed to get secret: %v", err)
	}

	namespaces := string(updatedSecret.Data[common.NamespaceKey])
	if namespaces != "other-ns" {
		t.Errorf("Expected namespaces to be 'other-ns', got %s", namespaces)
	}

	// Verify finalizer was removed
	if controllerutil.ContainsFinalizer(app, common.FinalizerName) {
		t.Error("Expected finalizer to be removed")
	}
}

func TestHandleDelete_KeepNamespaceWhenOtherAppsUseIt(t *testing.T) {
	cl, _ := setupTest(t)
	ctx := context.Background()
	log := logr.Discard()
	common.ServerUrlDomain = "example.com"

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster.example.com-cluster-tokentoken",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			common.NamespaceKey: []byte("test-dest-ns"),
		},
	}
	if err := cl.Create(ctx, secret); err != nil {
		t.Fatalf("Failed to create secret: %v", err)
	}

	app1 := &argoprojv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-app1",
			Namespace:  "test-ns",
			Finalizers: []string{common.FinalizerName},
		},
		Spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Server:    "test-cluster",
				Namespace: "test-dest-ns",
			},
		},
	}
	if err := cl.Create(ctx, app1); err != nil {
		t.Fatalf("Failed to create app1: %v", err)
	}

	app2 := &argoprojv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app2",
			Namespace: "test-ns",
		},
		Spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Server:    "test-cluster",
				Namespace: "test-dest-ns",
			},
		},
	}
	if err := cl.Create(ctx, app2); err != nil {
		t.Fatalf("Failed to create app2: %v", err)
	}

	err := HandleDelete(log, ctx, cl, app1)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify namespace was NOT removed from secret
	updatedSecret := &corev1.Secret{}
	err = cl.Get(ctx, client.ObjectKeyFromObject(secret), updatedSecret)
	if err != nil {
		t.Fatalf("Failed to get secret: %v", err)
	}

	namespaces := string(updatedSecret.Data[common.NamespaceKey])
	if namespaces != "test-dest-ns" {
		t.Errorf("Expected namespaces to remain 'test-dest-ns', got %s", namespaces)
	}

	// Verify finalizer was still removed
	if controllerutil.ContainsFinalizer(app1, common.FinalizerName) {
		t.Error("Expected finalizer to be removed")
	}
}

func TestHandleDelete_NoFinalizerNoOp(t *testing.T) {
	cl, _ := setupTest(t)
	ctx := context.Background()
	log := logr.Discard()

	app := &argoprojv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "test-ns",
			// No finalizers
		},
		Spec: argoprojv1alpha1.ApplicationSpec{
			Destination: argoprojv1alpha1.ApplicationDestination{
				Server:    "test-cluster",
				Namespace: "test-dest-ns",
			},
		},
	}

	err := HandleDelete(log, ctx, cl, app)
	if err != nil {
		t.Errorf("Expected no error for app without finalizer, got %v", err)
	}
}
