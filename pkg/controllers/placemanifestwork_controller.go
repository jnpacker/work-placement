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
	"errors"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	pd "open-cluster-management.io/api/cluster/v1alpha1"

	pmw "jnpacker/work-placement/api/v1alpha1"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	workapi "open-cluster-management.io/api/work/v1"
)

// PlaceManifestWorkReconciler reconciles a PlaceManifestWork object
type PlaceManifestWorkReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	ctx    context.Context
	Log    logr.Logger
}

const (
	destroyFinalizer           = "placemanifestwork.work.open-cluster-management.io/finalizer"
	PlacementLabel             = "cluster.open-cluster-management.io/placement"
	placementManifestworkLabel = "cluster.open-cluster-management.io/placementmanifestwork"
	LastReconcileTimestamp     = "placemanifestwork.work.open-cluster-management.io/lastPlacementTrigger"
)

//+kubebuilder:rbac:groups=work.open-cluster-management.io,resources=placementmanifestworks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=work.open-cluster-management.io,resources=placementmanifestworks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=work.open-cluster-management.io,resources=placementmanifestworks/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=placementdecisions,verbs=get;list;watch
//+kubebuilder:rbac:groups=work.open-cluster-management.io,resources=manifestworks,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PlaceManifestWork object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *PlaceManifestWorkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log = log.FromContext(ctx)
	r.ctx = ctx
	log := r.Log

	log.Info("Reconciling...")

	var pm pmw.PlaceManifestWork
	if err := r.Get(ctx, req.NamespacedName, &pm); err != nil {
		log.Info("Resource deleted")
		return ctrl.Result{}, nil
	}

	var pdList pd.PlacementDecisionList
	if pm.DeletionTimestamp != nil {
		pdList.Items = []pd.PlacementDecision{}
	} else {
		if pm.Spec.PlacementRef == nil {
			log.Error(errors.New("placementRef = nil"), "PlacementRef is not defined")
			return ctrl.Result{}, r.updateStatusConditionsOnChange(&pm, pmw.PlacementDecisionVerified, metav1.ConditionFalse, "PlacementRef is not defined", pmw.PlacementDecisionNotFound)
		}

		if err := r.List(ctx, &pdList, client.MatchingLabels{PlacementLabel: pm.Spec.PlacementRef.Name}, client.InNamespace(pm.Namespace)); err != nil || len(pdList.Items) == 0 {
			log.Error(err, "PlacementRef did not produce a valid PlacementDecision")
			return ctrl.Result{}, r.updateStatusConditionsOnChange(&pm, pmw.PlacementDecisionVerified, metav1.ConditionFalse, "PlacementRef did not produce a valid PlacementDecision", pmw.PlacementDecisionNotFound)
		}

		r.updateStatusConditionsOnChange(&pm, pmw.PlacementDecisionVerified, metav1.ConditionTrue, "", pmw.PlacementDecisionVerifiedAsExpected)

		// Set the finalizer if its not present, and store the placement name for reconciling
		if pm.ObjectMeta.Labels == nil {
			pm.ObjectMeta.Labels = map[string]string{
				placementManifestworkLabel: pm.Spec.PlacementRef.Name,
			}
		}

		if !controllerutil.ContainsFinalizer(&pm, destroyFinalizer) {
			if err := r.updateStatusConditionsOnChange(&pm, pmw.ManifestworkApplied, metav1.ConditionFalse, "Parsing and applying ManifestWorks", pmw.Processing); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("Add finalizer")
			inPm := pm.DeepCopy()
			controllerutil.AddFinalizer(&pm, destroyFinalizer)
			if err := r.Patch(ctx, &pm, client.MergeFrom(inPm)); err != nil {
				return ctrl.Result{}, err
			}
		}

		if pm.ObjectMeta.Labels[pd.PlacementLabel] != pm.Spec.PlacementRef.Name {
			log.Info("Update placementRef.name")
			inPm := pm.DeepCopy()
			pm.ObjectMeta.Labels[pd.PlacementLabel] = pm.Spec.PlacementRef.Name
			if err := r.Patch(ctx, &pm, client.MergeFrom(inPm)); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Check the first time decision list is > 0
		if len(pdList.Items[0].Status.Decisions) == 0 {
			return ctrl.Result{}, r.updateStatusConditionsOnChange(&pm, pmw.PlacementDecisionVerified, metav1.ConditionFalse, "PlacementRef has not made any decisions", pmw.PlacementDecisionEmpty)
		}
	}

	if pm.Status.ManifestWorkDelivery == nil {
		pm.Status.ManifestWorkDelivery = map[string]pmw.ManifestWorkDelivery{}
	}

	// Find all ManifestWork owned by this PlaceManifestWork
	var mwList workapi.ManifestWorkList
	if err := r.List(ctx, &mwList, client.MatchingLabels{placementManifestworkLabel: string(pm.UID)}); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "Problem retrieve ManifestWork")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Build a map by clusterName (unique)
	mwMap := map[string]workapi.ManifestWork{}
	for _, work := range mwList.Items {
		mwMap[work.Namespace] = work
	}

	// Reconcile all the ManifestWork resources
	// Loop through all placementDecisions
	for _, placementDecision := range pdList.Items {
		// Loop through all clusterNames in the status.decisions
		for _, clusterDecision := range placementDecision.Status.Decisions {
			work, found := mwMap[clusterDecision.ClusterName]
			if found {
				update := false
				/*if !reflect.DeepEqual(work.Spec.DeleteOption, pm.Spec.ManifestWorkSpec.DeleteOption) {
					work.Spec.DeleteOption = pm.Spec.ManifestWorkSpec.DeleteOption
					log.Info("Changing Spec.DeleteOption")
					update = true
				}
				if !reflect.DeepEqual(work.Spec.ManifestConfigs, pm.Spec.ManifestWorkSpec.ManifestConfigs) {
					work.Spec.ManifestConfigs = pm.Spec.ManifestWorkSpec.ManifestConfigs
					log.Info("Changing Spec.DeleteOption")
					update = true
				}*/
				if !reflect.DeepEqual(work.Spec, pm.Spec.ManifestWorkSpec) {
					work.Spec = pm.Spec.ManifestWorkSpec
					log.Info("Changing Spec")
					update = true
				}
				// Remove the key, so that anything remaining in the map can be deleted at the end
				delete(mwMap, clusterDecision.ClusterName)
				if update {
					if err := r.Update(ctx, &work); err != nil {
						return ctrl.Result{}, err
					}
				}

			} else {
				work := workapi.ManifestWork{
					ObjectMeta: metav1.ObjectMeta{Name: pm.Name, Namespace: clusterDecision.ClusterName,
						Labels: map[string]string{
							placementManifestworkLabel: string(pm.UID),
						},
					},
					Spec: pm.Spec.ManifestWorkSpec,
				}
				if err := r.Create(ctx, &work); err != nil {
					return ctrl.Result{}, err
				}
				log.Info("Created ManifestWork for cluster " + work.Namespace)
			}
			pm.Status.ManifestWorkDelivery[clusterDecision.ClusterName] = pmw.ManifestWorkDelivery{AppliedWork: true}
		}
	}

	// Remove any ManifestWork that were not found
	for _, work := range mwMap {
		if err := r.Delete(ctx, &work); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Deleted ManifestWork " + work.Name + " from cluster " + work.Namespace)
		pm.Status.ManifestWorkDelivery[work.Name] = pmw.ManifestWorkDelivery{AppliedWork: false}
	}

	if pm.DeletionTimestamp != nil {
		controllerutil.RemoveFinalizer(&pm, destroyFinalizer)
		if err := r.Update(ctx, &pm); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Removed finalizer")
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	// We want to reconcile our ManifestWork
	return ctrl.Result{RequeueAfter: 2 * time.Minute}, r.updateStatusConditionsOnChange(&pm, pmw.ManifestworkApplied, metav1.ConditionTrue, "", pmw.AsExpected)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PlaceManifestWorkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pmw.PlaceManifestWork{}).
		Complete(r)
}

// Helpers for modifying Status.Conditions
func (r *PlaceManifestWorkReconciler) updateStatusConditionsOnChange(pm *pmw.PlaceManifestWork, conditionType pmw.ConditionType, conditionStatus metav1.ConditionStatus, message string, reason string) error {

	var err error = nil
	sc := meta.FindStatusCondition(pm.Status.Conditions, string(conditionType))
	if sc == nil || sc.ObservedGeneration != pm.Generation || sc.Status != conditionStatus || sc.Reason != reason || sc.Message != message {
		inPm := pm.DeepCopy()
		setStatusCondition(pm, conditionType, conditionStatus, message, reason)
		err = r.Client.Status().Patch(r.ctx, pm, client.MergeFrom(inPm))
		if err != nil {
			if apierrors.IsConflict(err) {
				r.Log.Error(err, "Conflict encountered when updating PlacementManifestwork.Status")
			} else {
				r.Log.Error(err, "Failed to update PlacementManifestwork.Status")
			}
		}
	}
	return err
}

func setStatusCondition(pm *pmw.PlaceManifestWork, conditionType pmw.ConditionType, status metav1.ConditionStatus, message string, reason string) metav1.Condition {
	if pm.Status.Conditions == nil {
		pm.Status.Conditions = []metav1.Condition{}
	}
	condition := metav1.Condition{
		Type:               string(conditionType),
		ObservedGeneration: pm.Generation,
		Status:             status,
		Message:            message,
		Reason:             reason,
	}
	meta.SetStatusCondition(&pm.Status.Conditions, condition)
	return condition
}
