/*
Copyright 2022.

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

package controllers

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	pd "open-cluster-management.io/api/cluster/v1alpha1"

	pmw "jnpacker/work-placement/api/v1alpha1"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// PlacementReconciler reconciles a Placement object, and forces a PlaceManifestWork reconcile
type PlacementReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	ctx    context.Context
	Log    logr.Logger
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PlaceManifestWork object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *PlacementReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log = log.FromContext(ctx)
	r.ctx = ctx
	log := r.Log

	log.Info("Reconciling...")

	var p pd.PlacementDecision
	if err := r.Get(ctx, req.NamespacedName, &p); err != nil {
		log.Info("Resource not found")
		return ctrl.Result{}, nil
	}
	if p.ObjectMeta.Labels[PlacementLabel] == "" {
		log.Info("PlacementDecision missing label: " + PlacementLabel)
		return ctrl.Result{}, nil
	}

	var pmwList pmw.PlaceManifestWorkList
	// This guarantees that a PlaceManifestWork will not be missed, but it is slower then using the label
	//if err := r.List(ctx, &pmwList, client.MatchingLabels{PlacementLabel: p.Name}, client.InNamespace(p.Namespace)); err != nil && !apierrors.IsNotFound(err) {
	if err := r.List(ctx, &pmwList, client.InNamespace(p.Namespace)); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "Problem retrieving PlaceManifestWorks")
		return ctrl.Result{}, nil
	}

	for _, placeManifestWork := range pmwList.Items {
		if placeManifestWork.Spec.PlacementRef != nil &&
			placeManifestWork.Spec.PlacementRef.Name == p.Name {
			log.Info("Trigger reconcile for PlaceManifestWork " + placeManifestWork.Name)
			placeManifestWork.ObjectMeta.Annotations[LastReconcileTimestamp] = time.Now().Format(time.RFC3339)
			if err := r.Update(ctx, &placeManifestWork); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("Triggered reconcile for PlaceManifestWork " + placeManifestWork.Name)
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PlacementReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pd.PlacementDecision{}).
		Complete(r)
}
