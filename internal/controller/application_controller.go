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

package controller

import (
	"context"
	"strings"

	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dana-team/application-rbac-validator/internal/controller/handlers"
	"github.com/dana-team/application-rbac-validator/internal/utils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	NamespacePrefix string
}

// +kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=argoproj.io,resources=applications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=argoproj.io,resources=applications/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;update;patch

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	baseLogger := zap.New().WithName("controller")
	app := &argoprojv1alpha1.Application{}
	if err := r.Get(ctx, req.NamespacedName, app); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		baseLogger.Error(err, "unable to fetch Application")
		return ctrl.Result{}, err
	}
	log := baseLogger.WithValues("app", app.Name, "destination", app.Spec.Destination.Server)
	if utils.IsInCluster(app.Spec.Destination.Server) {
		log.Info("application is targeting in-cluster, ignoring...", "app", app.Name)
		return ctrl.Result{}, nil
	}

	if !app.ObjectMeta.DeletionTimestamp.IsZero() {
		log.Info("application is being deleted, cleaning up...")
		return ctrl.Result{}, handlers.HandleDelete(log, ctx, r.Client, app)
	}
	return ctrl.Result{}, handlers.HandleCreateOrUpdate(log, ctx, r.Client, app)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&argoprojv1alpha1.Application{}).
		Watches(&argoprojv1alpha1.Application{},
			handler.EnqueueRequestsFromMapFunc(
				func(ctx context.Context, o client.Object) []reconcile.Request {
					namespace := o.GetNamespace()
					if len(r.NamespacePrefix) > 0 && len(namespace) > len(r.NamespacePrefix) && strings.HasPrefix(namespace, r.NamespacePrefix) {
						return []reconcile.Request{
							{NamespacedName: client.ObjectKey{
								Name:      o.GetName(),
								Namespace: namespace,
							},
							},
						}
					}
					return []reconcile.Request{}
				},
			),
		).
		Complete(r)
}
