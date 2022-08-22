package controllers

import (
	"context"

	"github.com/pkg/errors"
	externalv1 "github.com/platform9/cluster-api-provider-external/api/infrastructure/v1beta1"
	"github.com/platform9/cluster-api-provider-external/pkg/scope"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// ClusterFinalizer allows ReconcileExternalCluster to clean up External resources
	// associated with ExternalCluster before removing it from the apiserver.
	ClusterFinalizer               = "externalcluster.infrastructure.cluster.x-k8s.io"
	ReadyCondition                 = "Ready"
	KubeconfigSecretNotFoundReason = "KubeconfigSecretNotFound"
	KubeconfigInvalidReason        = "KubeconfigInvalid"
	ClusterAccessFailedReason      = "ClusterAccessFailed"
	NodesListFailedReason          = "NodesListFailed"
)

// ExternalClusterReconciler reconciles a ExternalCluster object
type ExternalClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExternalClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&externalv1.ExternalCluster{}).
		WithEventFilter(predicates.ResourceNotPaused(ctrl.LoggerFrom(ctx))). // don't queue reconcile if resource is paused
		Build(r)
	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}

	// Add a watch on clusterv1.Cluster object for unpause notifications.
	if err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(externalv1.GroupVersion.WithKind("ExternalCluster"))),
		predicates.ClusterUnpaused(ctrl.LoggerFrom(ctx)),
	); err != nil {
		return errors.Wrapf(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=externalclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=externalclusters/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ExternalCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *ExternalClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Fetching ExternalCluster from storage")
	var externalCluster externalv1.ExternalCluster
	if err := r.Get(ctx, req.NamespacedName, &externalCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, externalCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("OwnerCluster is not set yet. Requeuing...")
		return ctrl.Result{}, nil
	}

	if annotations.IsPaused(cluster, &externalCluster.ObjectMeta) {
		log.Info("ExternalCluster or linked Cluster is marked as paused. Won't reconcile")
		return reconcile.Result{}, nil
	}

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Logger:          log,
		Client:          r.Client,
		Cluster:         cluster,
		ExternalCluster: &externalCluster,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	defer func() {
		if err := clusterScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
		log.Info("Cluster reconciled.")
	}()

	// Handle deleted clusters
	if !cluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, clusterScope)
	}
	return r.reconcileNormal(ctx, clusterScope)
}

func (r *ExternalClusterReconciler) reconcileNormal(ctx context.Context, clusterScope *scope.ExternalClusterScope) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	// externalCluster := clusterScope.ExternalCluster
	// controllerutil.AddFinalizer(externalCluster, ClusterFinalizer)

	// Reconcile the kubeconfig secret
	log.V(4).Info("Fetching the external cluster kubeconfig from the associated")
	rawKubeconfig, err := kubeconfig.FromSecret(ctx, r.Client, clusterScope.NamespacedName())
	if err != nil {
		conditions.MarkFalse(clusterScope.ExternalCluster, ReadyCondition, KubeconfigSecretNotFoundReason, clusterv1.ConditionSeverityInfo, err.Error())
		return ctrl.Result{}, err
	}
	clusterConfig, err := clientcmd.RESTConfigFromKubeConfig(rawKubeconfig)
	if err != nil {
		conditions.MarkFalse(clusterScope.ExternalCluster, ReadyCondition, KubeconfigInvalidReason, clusterv1.ConditionSeverityInfo, err.Error())
		return ctrl.Result{}, err
	}
	clusterClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		conditions.MarkFalse(clusterScope.ExternalCluster, ReadyCondition, KubeconfigInvalidReason, clusterv1.ConditionSeverityInfo, err.Error())
		return ctrl.Result{}, err
	}

	log.V(4).Info("Checking if the cluster is accessible")
	_, err = clusterClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		conditions.MarkFalse(clusterScope.ExternalCluster, ReadyCondition, ClusterAccessFailedReason, clusterv1.ConditionSeverityInfo, err.Error())
		return ctrl.Result{}, err
	}

	log.V(4).Info("Retrieving nodes from external cluster")
	nodes, err := clusterClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		conditions.MarkFalse(clusterScope.ExternalCluster, ReadyCondition, NodesListFailedReason, clusterv1.ConditionSeverityInfo, err.Error())
		return ctrl.Result{}, err
	}

	log.V(4).Info("Syncing external machines with the nodes in the external cluster")
	for _, node := range nodes.Items {
		machine := convertNodeToMachine(clusterScope.Cluster, &node)

		err := r.Client.Create(ctx, machine)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, err
		}
		err = r.Client.Create(ctx, machine)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, err
		}
		err = r.Client.Create(ctx, machine)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, err
		}
	}

	// TODO actually check if it is ready
	clusterScope.ExternalCluster.Status.Ready = true
	return ctrl.Result{}, nil
}

func (r *ExternalClusterReconciler) reconcileDelete(ctx context.Context, clusterScope *scope.ExternalClusterScope) (ctrl.Result, error) {
	// controllerutil.RemoveFinalizer(clusterScope.ExternalCluster, ClusterFinalizer)
	return ctrl.Result{}, nil
}

func convertNodeToMachine(cluster *clusterv1.Cluster, node *corev1.Node) *clusterv1.Machine {
	machineName := node.Name
	return &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machineName,
			Namespace: cluster.Namespace,
		},
		Spec: clusterv1.MachineSpec{
			ClusterName: cluster.Name,
			Bootstrap:   clusterv1.Bootstrap{}, // TODO can we do without?
			InfrastructureRef: corev1.ObjectReference{
				Kind: "ExternalMachine",
				Name: machineName,
			},
			Version:    &node.Status.NodeInfo.KubeletVersion,
			ProviderID: &node.Spec.ProviderID,
		},
		Status: clusterv1.MachineStatus{},
	}
}
