package controllers

import (
	"context"

	"github.com/pkg/errors"
	externalv1 "github.com/platform9-incubator/cluster-api-provider-external/api/infrastructure/v1beta1"
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

// ExternalMachineReconciler reconciles a ExternalMachine object
type ExternalMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExternalMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&externalv1.ExternalMachine{}).
		WithEventFilter(predicates.ResourceNotPaused(ctrl.LoggerFrom(ctx))). // don't queue reconcile if resource is paused
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(externalv1.GroupVersion.WithKind("ExternalMachine"))),
		).
		Build(r)
	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}

	// Add a watch on clusterv1.Machine object for unpause notifications.
	if err = c.Watch(
		&source.Kind{Type: &clusterv1.Machine{}},
		handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(externalv1.GroupVersion.WithKind("ExternalMachine"))),
		predicates.ClusterUnpaused(ctrl.LoggerFrom(ctx)),
	); err != nil {
		return errors.Wrapf(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=externalmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=externalmachines/status,verbs=get;update;patch

func (r *ExternalMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Fetching ExternalMachine from storage")
	var externalMachine externalv1.ExternalMachine
	if err := r.Get(ctx, req.NamespacedName, &externalMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, externalMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("OwnerMachine is not set yet. Requeuing...")
		return ctrl.Result{}, nil
	}
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Info("Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	if annotations.IsPaused(cluster, &externalMachine.ObjectMeta) {
		log.Info("ExternalMachine or linked Cluster is marked as paused. Won't reconcile")
		return reconcile.Result{}, nil
	}

	// Create the machine scope
	machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
		Logger:          log,
		Client:          r.Client,
		Machine:         machine,
		Cluster:         cluster,
		ExternalMachine: &externalMachine,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	defer func() {
		if err := machineScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
		log.Info("Machine reconciled.")
	}()

	// Handle deleted clusters
	if !machine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineScope)
	}
	return r.reconcileNormal(ctx, machineScope)
}

func (r *ExternalMachineReconciler) reconcileNormal(ctx context.Context, clusterScope *scope.ExternalMachineScope) (ctrl.Result, error) {
	// log := ctrl.LoggerFrom(ctx)
	// externalMachine := clusterScope.ExternalMachine
	// controllerutil.AddFinalizer(externalMachine, MachineFinalizer)
	// TODO actually check if it is ready
	clusterScope.ExternalMachine.Status.Ready = true
	return ctrl.Result{}, nil
}

func (r *ExternalMachineReconciler) reconcileDelete(ctx context.Context, clusterScope *scope.ExternalMachineScope) (ctrl.Result, error) {
	// controllerutil.RemoveFinalizer(clusterScope.ExternalMachine, MachineFinalizer)
	return ctrl.Result{}, nil
}
