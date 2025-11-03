package handlers

import (
	"context"
	"slices"

	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dana-team/application-rbac-validator/internal/common"
	"github.com/dana-team/application-rbac-validator/internal/metrics"
	"github.com/dana-team/application-rbac-validator/internal/utils"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

	if utils.ShouldBypassOptimization(secret) {
		log.Info("Bypass optimization label exists on destination secret, skipping ...", "app", app.Name, "cluster", app.Spec.Destination.Server)
		metrics.ObserveApplicationOptimizationStatus(app.Name, app.Namespace, app.Spec.Destination.Server, "bypass-label", false)
		return nil
	}

	namespaceList := utils.ExtractNamespacesFromSecret(secret)
	if destinationNS != "" && !slices.Contains(namespaceList, destinationNS) {
		err = utils.RetryUpdateSecret(ctx, cl, app, append(namespaceList, destinationNS))
		if err != nil {
			log.Error(err, "Failed to update secret", "secretName", secret.Name, "namespace", secret.Namespace, "destinationNS", destinationNS)
			return err
		}

		log.Info("Updated secret with new namespace", "secretName", secret.Name, "namespace", destinationNS)
	}

	metrics.ObserveApplicationOptimizationStatus(app.Name, app.Namespace, app.Spec.Destination.Server, "optimized", true)

	return nil

}

func IsClusterWide(secret *corev1.Secret) bool {
	clusterWide := false
	clusterResourcesRaw, ok := secret.Data[common.ClusterResourcesKey]
	if ok {
		clusterWide = string(clusterResourcesRaw) == "true"
	}
	return clusterWide
}

// HandleDelete handles the deletion of an Application resource.
func HandleDelete(log logr.Logger, ctx context.Context, cl client.Client, app *argoprojv1alpha1.Application) error {
	if utils.IsInCluster(app.Spec.Destination.Server) {
		log.Info("application is targeting in-cluster, ignoring...", "app", app.Name)
		metrics.ObserveApplicationOptimizationStatus(app.Name, app.Namespace, app.Spec.Destination.Server, "in-cluster", false)
		return nil
	}
	secret, err := utils.FetchDestinationClusterSecret(ctx, cl, app)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("secret not found, skipping namespace cleanup", "app", app.Name)
			metrics.DeleteApplicationOptimizationStatus(app.Name, app.Namespace, app.Spec.Destination.Server)
			return nil
		}
		log.Error(err, "Failed to fetch secret for application", "app", app.Name)
		return err
	}
	if utils.ShouldBypassOptimization(secret) {
		log.Info("Destination secret has bypass label, skipping...", "app", app.Name, "cluster", app.Spec.Destination.Server)
		metrics.DeleteApplicationOptimizationStatus(app.Name, app.Namespace, app.Spec.Destination.Server)
		return nil
	}
	destinationNS := app.Spec.Destination.Namespace
	applicationList := &argoprojv1alpha1.ApplicationList{}
	if err := cl.List(ctx, applicationList, &client.ListOptions{Namespace: app.Namespace}); err != nil {
		log.Error(err, "Failed to list applications in namespace", "namespace", app.Namespace)
		return err
	}
	if !utils.IsDestinationNamespaceInUse(applicationList, app, destinationNS) && destinationNS != "" {
		namespaceList := utils.ExtractNamespacesFromSecret(secret)
		// Remove the namespace from the list
		var newNamespaceList []string
		for _, ns := range namespaceList {
			if ns != destinationNS {
				newNamespaceList = append(newNamespaceList, ns)
			}
		}

		err = utils.RetryUpdateSecret(ctx, cl, app, newNamespaceList)
		if err != nil {
			log.Error(err, "Failed to update secret", "secretName", secret.Name, "namespace", destinationNS)
			return err
		}

		log.Info("Removed namespace from secret", "secretName", secret.Name, "namespace", destinationNS)
	}
	metrics.DeleteApplicationOptimizationStatus(app.Name, app.Namespace, app.Spec.Destination.Server)
	return nil

}
