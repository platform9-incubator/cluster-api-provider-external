package controllers

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	externalv1 "github.com/platform9-incubator/cluster-api-provider-external/api/controlplane/v1beta1"
	"github.com/platform9-incubator/cluster-api-provider-external/pkg/scope"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// ControlPlaneFinalizer allows ReconcileExternalControlPlane to clean up External resources
	// associated with ExternalControlPlane before removing it from the apiserver.
	ControlPlaneFinalizer = "external.controlplane.cluster.x-k8s.io"
)

// ExternalControlPlaneReconciler reconciles a ExternalControlPlane object
type ExternalControlPlaneReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExternalControlPlaneReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&externalv1.ExternalControlPlane{}).
		WithEventFilter(predicates.ResourceNotPaused(ctrl.LoggerFrom(ctx))). // don't queue reconcile if resource is paused
		Build(r)
	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}

	// Add a watch on clusterv1.Cluster object for unpause notifications.
	err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(r.ClusterToExternalControlPlane),
		predicates.All(ctrl.LoggerFrom(ctx),
			// predicates.ResourceHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue),
			predicates.ClusterUnpausedAndInfrastructureReady(ctrl.LoggerFrom(ctx)),
		),
	)
	if err != nil {
		return errors.Wrap(err, "failed adding Watch for Clusters to controller manager")
	}

	return nil
}

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=externalcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=externalcontrolplanes/status,verbs=get;update;patch

func (r *ExternalControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Fetching ExternalControlPLane from storage")
	var externalControlPlane externalv1.ExternalControlPlane
	if err := r.Get(ctx, req.NamespacedName, &externalControlPlane); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the ControlPlane.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, externalControlPlane.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("OwnerControlPlane is not set yet. Requeuing...")
		return ctrl.Result{}, nil
	}

	if annotations.IsPaused(cluster, &externalControlPlane.ObjectMeta) {
		log.Info("ExternalControlPlane or linked ControlPlane is marked as paused. Won't reconcile")
		return reconcile.Result{}, nil
	}

	// Create the cluster scope
	clusterScope, err := scope.NewControlPlaneScope(scope.ControlPlaneScopeParams{
		Logger:               log,
		Client:               r.Client,
		Cluster:              cluster,
		ExternalControlPlane: &externalControlPlane,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	defer func() {
		if err := clusterScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
		log.Info("ControlPlane reconciled.")
	}()

	// Handle deleted clusters
	if !cluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, clusterScope)
	}
	return r.reconcileNormal(ctx, clusterScope)
}

func (r *ExternalControlPlaneReconciler) reconcileNormal(ctx context.Context, clusterScope *scope.ControlPlaneScope) (ctrl.Result, error) {
	// externalControlPlane := clusterScope.ExternalControlPlane
	// controllerutil.AddFinalizer(externalControlPlane, ControlPlaneFinalizer)
	// TODO actually check if it is ready
	clusterScope.ExternalControlPlane.Status.Ready = true
	clusterScope.ExternalControlPlane.Status.Initialized = true
	return ctrl.Result{}, nil
}

func (r *ExternalControlPlaneReconciler) reconcileDelete(ctx context.Context, clusterScope *scope.ControlPlaneScope) (ctrl.Result, error) {
	// controllerutil.RemoveFinalizer(clusterScope.ExternalControlPlane, ControlPlaneFinalizer)
	return ctrl.Result{}, nil
}

// ClusterToExternalControlPlane is a handler.ToRequestsFunc to be used to enqueue requests for reconciliation
// for ExternalControlPlane based on updates to a Cluster.
func (r *ExternalControlPlaneReconciler) ClusterToExternalControlPlane(o client.Object) []ctrl.Request {
	c, ok := o.(*clusterv1.Cluster)
	if !ok {
		panic(fmt.Sprintf("Expected a Cluster but got a %T", o))
	}

	controlPlaneRef := c.Spec.ControlPlaneRef
	if controlPlaneRef != nil && controlPlaneRef.Kind == "ExternalControlPlane" {
		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: controlPlaneRef.Namespace, Name: controlPlaneRef.Name}}}
	}

	return nil
}
