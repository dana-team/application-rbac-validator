package handlers

import (
	"context"
	"fmt"
	"slices"
	"strings"

	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/dana-team/application-rbac-validator/internal/common"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func HandleCreateOrUpdate(log logr.Logger, ctx context.Context, cl client.Client, app *argoprojv1alpha1.Application) error {
	destinationNS := app.Spec.Destination.Namespace
	if common.IsInCluster(app.Spec.Destination.Server) {
		log.Info("application is targeting in-cluster, not optimizing", "app", app.Name)
		return nil
	}
	secret, err := common.FetchSecretFromApplication(ctx, cl, app)
	if err != nil {
		log.Error(err, "Failed to fetch secret for application", "app", app.Name)
		return err
	}
	namespaceList := common.GetNamespacesList(secret)
	clusterWide := false
	clusterResourcesRaw, ok := secret.Data[common.ClusterResourcesKey]
	if ok {
		clusterWide = string(clusterResourcesRaw) == "true"
	}
	if clusterWide {
		log.Info("Clusterwide enabled, not optimizing", "app", app.Name, "cluster", app.Spec.Destination.Server)
		return nil
	}
	if destinationNS != "" && !slices.Contains(namespaceList, destinationNS) {
		secret.Data[common.NamespaceKey] = []byte(fmt.Sprintf("%s,%s", strings.Join(namespaceList, ","), destinationNS))
		if err := cl.Update(ctx, secret); err != nil {
			log.Error(err, "Failed to update secret", "secretName", secret.Name, "namespace", app.Name)
			return err
		}
		log.Info("Updated secret with new namespace", "secretName", secret.Name, "namespace", destinationNS)
	}
	return ensureFinalizer(log, ctx, cl, app)

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

func HandleDelete(log logr.Logger, ctx context.Context, cl client.Client, app *argoprojv1alpha1.Application) error {
	if !controllerutil.ContainsFinalizer(app, common.FinalizerName) {
		return nil
	}
	secret, err := common.FetchSecretFromApplication(ctx, cl, app)
	if err != nil {
		log.Error(err, "Failed to fetch secret for application", "app", app.Name)
		return err
	}
	destinationNS := app.Spec.Destination.Namespace
	applicationList := &argoprojv1alpha1.ApplicationList{}
	if err := cl.List(ctx, applicationList, &client.ListOptions{Namespace: app.Namespace}); err != nil {
		log.Error(err, "Failed to list applications in namespace", "namespace", app.Namespace)
		return err
	}
	// Check if any other application is using the same namespace
	otherAppUsingNS := false
	for _, otherApp := range applicationList.Items {
		if otherApp.Name != app.Name && otherApp.Spec.Destination.Namespace == destinationNS && otherApp.Spec.Destination.Server == app.Spec.Destination.Server {
			otherAppUsingNS = true
			break
		}
	}
	if !otherAppUsingNS && destinationNS != "" {
		namespaceList := common.GetNamespacesList(secret)
		// Remove the namespace from the list
		newNamespaceList := []string{}
		for _, ns := range namespaceList {
			if ns != destinationNS {
				newNamespaceList = append(newNamespaceList, ns)
			}
		}
		secret.Data[common.NamespaceKey] = []byte(strings.Join(newNamespaceList, ","))
		if err := cl.Update(ctx, secret); err != nil {
			log.Error(err, "Failed to update secret", "secretName", secret.Name, "namespace", app.Name)
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
