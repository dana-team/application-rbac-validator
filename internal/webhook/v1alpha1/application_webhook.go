/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"
	"fmt"

	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dana-team/application-rbac-validator/internal/common"
	"github.com/dana-team/application-rbac-validator/internal/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupApplicationWebhookWithManager registers the webhook for Application in the manager.
func SetupApplicationWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&argoprojv1alpha1.Application{}).
		WithValidator(&ApplicationCustomValidator{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:rbac:groups="",resources=configmaps;namespaces,verbs=get;list;watch
// +kubebuilder:webhook:path=/validate-argoproj-io-v1alpha1-application,mutating=false,failurePolicy=fail,sideEffects=None,groups=argoproj.io,resources=applications,verbs=create;update,versions=v1alpha1,name=vapplication-v1alpha1.kb.io,admissionReviewVersions=v1

type ApplicationCustomValidator struct {
	Client                   client.Client
	destinationClusterClient kubernetes.Interface
}

var ClusterDomain string

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Application.
func (v *ApplicationCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	logger := zap.New().WithName("webhook")
	application, ok := obj.(*argoprojv1alpha1.Application)
	if !ok {
		return nil, fmt.Errorf("expected a Application object but got %T", obj)
	}
	logger.Info("Validation for Application upon creation", "name", application.GetName())

	return nil, validateApplication(ctx, v.Client, v.destinationClusterClient, application)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Application.
func (v *ApplicationCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	logger := zap.New().WithName("webhook")
	newApplication, ok := newObj.(*argoprojv1alpha1.Application)
	if !ok {
		return nil, fmt.Errorf("expected a Application object for the newObj but got %T", newObj)
	}
	oldApplication, ok := oldObj.(*argoprojv1alpha1.Application)
	if !ok {
		return nil, fmt.Errorf("expected a Application object for the oldObj but got %T", oldObj)
	}

	logger.Info("Validation for Application upon update", "name", newApplication.GetName())

	if utils.IsNotSpecUpdate(oldApplication, newApplication) {
		logger.V(1).Info("Only a status update, approving automatically.")
		return nil, nil
	}

	return nil, validateApplication(ctx, v.Client, v.destinationClusterClient, newApplication)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Application.
func (v *ApplicationCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// validateApplication prevents unauthorized application deployments across clusters or namespaces.
func validateApplication(ctx context.Context, k8sClient client.Client, destinationClusterClient kubernetes.Interface,
	application *argoprojv1alpha1.Application) error {
	logger := zap.New().WithName("webhook")
	destServer := application.Spec.Destination.Server
	destNamespace := application.Spec.Destination.Namespace
	appNamespace := application.GetNamespace()

	logger = logger.WithValues(
		"destinationServer", destServer,
	)

	if destNamespace == "" || destServer == "" {
		return fmt.Errorf("destination namespace and server must be specified")
	}

	utils.FetchClusterDomain()
	if !common.DomainEnvVarFound {
		logger.Info(fmt.Sprintf("Failed to fetch env var %s, validation might fail if server is not a URL", common.ClusterDomainEnvVar))
	}

	logger.Info("Checking if bypass label exists on the Application's namespace")
	isBypassLabelExists, err := utils.BypassLabelExists(ctx, k8sClient, appNamespace, utils.ExtractClusterName(destServer))
	if err != nil {
		return fmt.Errorf("failed to check bypass label on the Application's namespace: %w", err)
	}
	if isBypassLabelExists {
		logger.Info("Application approved")
		return nil
	}

	logger.Info("Checking if its a management Application")

	argoInstanceName, err := utils.FetchArgoInstanceName(ctx, k8sClient, appNamespace)
	if err != nil {
		return fmt.Errorf("failed to fetch Application's argo instance name: %w", err)
	}

	isManagementApplication := utils.IsManagementApplication(argoInstanceName, application.Name)

	if isManagementApplication {
		logger.Info("Application approved")
		return nil
	}

	logger.Info("Ensuring the Application's server and the destination server are not the same")

	if utils.IsInCluster(destServer) {
		return fmt.Errorf("destination server must not be the same as the Application's current cluster")
	}

	logger.Info("Fetching the webhook's current namespace name")

	currentNamespace, err := utils.GetCurrentNamespace()
	if err != nil {
		return fmt.Errorf("failed to fetch the webhook's current namespace name: %w", err)
	}

	logger.Info("Building destination Server url")

	if !utils.ValidateServerUrlFormat(destServer) {
		destServer = utils.BuildServerUrl(destServer)
	}

	logger.Info("Fetching destination cluster token")

	token, err := utils.FetchClusterToken(ctx, k8sClient, currentNamespace, destServer)
	if err != nil {
		return fmt.Errorf("failed to fetch cluster token: %w", err)
	}

	logger.Info("Accessing destination cluster")

	if destinationClusterClient == nil {
		destinationClusterClient, err = utils.BuildClusterClient(destServer, token)
		if err != nil {
			return fmt.Errorf("failed to build destination's cluster client: %w", err)
		}
	}

	logger.Info("Fetching authorized administrators for the Application's target environment.")

	admins, err := utils.FetchArgoInstanceUsers(ctx, k8sClient, appNamespace)
	if err != nil {
		return fmt.Errorf("failed to fetch Application's admins: %w", err)
	}

	logger.Info("Validating namespace access for account", "account", admins, "namespace", destNamespace, "cluster", destServer)

	if err := utils.EnsureAnyAdminHasNamespaceAccess(ctx, destinationClusterClient, admins, destNamespace, destServer); err != nil {
		return err
	}

	logger.Info("Application approved")

	return nil
}
