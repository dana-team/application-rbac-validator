package handlers

import (
	"context"
	"slices"
	"strings"

	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dana-team/application-rbac-validator/internal/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/dana-team/application-rbac-validator/internal/common"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HandleCreateOrUpdate handles the creation or update of an Application resource.
func HandleCreateOrUpdate(log logr.Logger, ctx context.Context, cl client.Client, app *argoprojv1alpha1.Application) error {
	destinationNS := app.Spec.Destination.Namespace
	secret, err := utils.FetchDestinationClusterSecret(ctx, cl, app)
	if err != nil {
		log.Error(err, "Failed to fetch secret for application", "app", app.Name)
		return err
	}
	namespaceList := utils.ExtractNamespacesFromSecret(secret)
	if IsClusterWide(secret) {
		log.Info("Clusterwide enabled, not optimizing", "app", app.Name, "cluster", app.Spec.Destination.Server)
		return nil
	}
	if destinationNS != "" && !slices.Contains(namespaceList, destinationNS) {
		namespaceList = append(namespaceList, destinationNS)
		secret.Data[common.NamespaceKey] = []byte(strings.Join(namespaceList, ","))
		if err := cl.Update(ctx, secret); err != nil {
			log.Error(err, "Failed to update secret", "secretName", secret.Name, "namespace", destinationNS)
			return err
		}
		log.Info("Updated secret with new namespace", "secretName", secret.Name, "namespace", destinationNS)
	}
	// NOTE: I am not currently implementing this, but we could also only add a finalizer if the namespace is added to the secret,
	// and remove it if the namespace is removed and no other application is using it. This would reduce the number of finalizers
	// could increase performance when deleting applications. But would add complexity. We need to consider if this is worth it.
	return ensureFinalizer(log, ctx, cl, app)

}

func IsClusterWide(secret *corev1.Secret) bool {
	clusterWide := false
	clusterResourcesRaw, ok := secret.Data[common.ClusterResourcesKey]
	if ok {
		clusterWide = string(clusterResourcesRaw) == "true"
	}
	return clusterWide
}

// ensureFinalizer ensures that the finalizer is present on the Application resource.
func ensureFinalizer(log logr.Logger, ctx context.Context, cl client.Client, app *argoprojv1alpha1.Application) error {
	if !controllerutil.ContainsFinalizer(app, common.FinalizerName) {
		controllerutil.AddFinalizer(app, common.FinalizerName)
		if err := cl.Update(ctx, app); err != nil {
			log.Error(err, "Failed to add finalizer to application", "app", app.Name)
			return err
		}
	}
	return nil
}

// HandleDelete handles the deletion of an Application resource.
func HandleDelete(log logr.Logger, ctx context.Context, cl client.Client, app *argoprojv1alpha1.Application) error {
	if !controllerutil.ContainsFinalizer(app, common.FinalizerName) {
		return nil
	}
	secret, err := utils.FetchDestinationClusterSecret(ctx, cl, app)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("secret not found, skipping namespace cleanup", "app", app.Name)
			controllerutil.RemoveFinalizer(app, common.FinalizerName)
			if err := cl.Update(ctx, app); err != nil {
				log.Error(err, "Failed to remove finalizer from application", "app", app.Name)
				return err
			}
			return nil
		}
		log.Error(err, "Failed to fetch secret for application", "app", app.Name)
		return err
	}
	destinationNS := app.Spec.Destination.Namespace
	applicationList := &argoprojv1alpha1.ApplicationList{}
	if err := cl.List(ctx, applicationList, &client.ListOptions{Namespace: app.Namespace}); err != nil {
		log.Error(err, "Failed to list applications in namespace", "namespace", app.Namespace)
		return err
	}
	if !isNamespaceInUse(applicationList, app, destinationNS) && destinationNS != "" {
		namespaceList := utils.ExtractNamespacesFromSecret(secret)
		// Remove the namespace from the list
		var newNamespaceList []string
		for _, ns := range namespaceList {
			if ns != destinationNS {
				newNamespaceList = append(newNamespaceList, ns)
			}
		}
		secret.Data[common.NamespaceKey] = []byte(strings.Join(newNamespaceList, ","))
		if err := cl.Update(ctx, secret); err != nil {
			log.Error(err, "Failed to update secret", "secretName", secret.Name, "namespace", destinationNS)
			return err
		}
		log.Info("Removed namespace from secret", "secretName", secret.Name, "namespace", destinationNS)
	}
	controllerutil.RemoveFinalizer(app, common.FinalizerName)
	if err := cl.Update(ctx, app); err != nil {
		log.Error(err, "Failed to remove finalizer from application", "app", app.Name)
		return err
	}
	return nil

}

// isNamespaceInUse checks if any other application is deploying to the same namespace in the same cluster.
func isNamespaceInUse(applicationList *argoprojv1alpha1.ApplicationList, app *argoprojv1alpha1.Application, destinationNS string) bool {
	otherAppUsingNS := false
	for _, otherApp := range applicationList.Items {
		if otherApp.Name != app.Name && otherApp.Spec.Destination.Namespace == destinationNS && otherApp.Spec.Destination.Server == app.Spec.Destination.Server {
			otherAppUsingNS = true
			break
		}
	}
	return otherAppUsingNS
}
