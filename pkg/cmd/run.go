package cmd

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/erwinvaneyk/cobras"
	externalcontrolplanev1 "github.com/platform9-incubator/cluster-api-provider-external/api/controlplane/v1beta1"
	externalinfrav1 "github.com/platform9-incubator/cluster-api-provider-external/api/infrastructure/v1beta1"
	"github.com/platform9-incubator/cluster-api-provider-external/controllers"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type RunOptions struct {
	*RootOptions
	KubeconfigPath              string
	metricsBindAddr             string
	enableLeaderElection        bool
	leaderElectionLeaseDuration time.Duration
	leaderElectionRenewDeadline time.Duration
	leaderElectionRetryPeriod   time.Duration
	watchNamespace              string
	syncPeriod                  time.Duration
	webhookPort                 int
	webhookCertDir              string
	healthAddr                  string
	profilerAddress             string
	watchFilterValue            string
	zapOpts                     zap.Options
}

func NewCmdRun(rootOptions *RootOptions) *cobra.Command {
	opts := &RunOptions{
		RootOptions:                 rootOptions,
		metricsBindAddr:             "localhost:8080",
		leaderElectionLeaseDuration: 1 * time.Minute,
		leaderElectionRenewDeadline: 40 * time.Second,
		leaderElectionRetryPeriod:   5 * time.Second,
		syncPeriod:                  10 * time.Minute,
		webhookPort:                 9443,
		webhookCertDir:              "/tmp/k8s-webhook-server/serving-certs/",
		healthAddr:                  ":9440",
		zapOpts:                     zap.Options{Development: true},
	}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the controllers.",
		Run:   cobras.Run(opts),
	}

	cmd.Flags().StringVar(&opts.metricsBindAddr, "metrics-bind-addr", opts.metricsBindAddr,
		"The address the metric endpoint binds to.")
	cmd.Flags().BoolVar(&opts.enableLeaderElection, "leader-elect", opts.enableLeaderElection,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	cmd.Flags().DurationVar(&opts.leaderElectionLeaseDuration, "leader-elect-lease-duration", opts.leaderElectionLeaseDuration,
		"Interval at which non-leader candidates will wait to force acquire leadership (duration string)")
	cmd.Flags().DurationVar(&opts.leaderElectionRenewDeadline, "leader-elect-renew-deadline", opts.leaderElectionRenewDeadline,
		"Duration that the leading controller manager will retry refreshing leadership before giving up (duration string)")
	cmd.Flags().DurationVar(&opts.leaderElectionRetryPeriod, "leader-elect-retry-period", opts.leaderElectionRetryPeriod,
		"Duration the LeaderElector clients should wait between tries of actions (duration string)")
	cmd.Flags().StringVar(&opts.watchNamespace, "namespace", opts.watchNamespace,
		"Namespace that the controller watches to reconcile cluster-api objects. If unspecified, the controller watches for cluster-api objects across all namespaces.")
	cmd.Flags().StringVar(&opts.profilerAddress, "profiler-address", opts.profilerAddress,
		"Bind address to expose the pprof profiler (e.g. localhost:6060)")
	cmd.Flags().DurationVar(&opts.syncPeriod, "sync-period", opts.syncPeriod,
		"The minimum interval at which watched resources are reconciled (e.g. 15m)")
	cmd.Flags().StringVar(&opts.watchFilterValue, "watch-filter", opts.watchFilterValue,
		fmt.Sprintf("Label value that the controller watches to reconcile cluster-api objects. Label key is always %s. If unspecified, the controller watches for all cluster-api objects.", clusterv1.WatchLabel))
	cmd.Flags().IntVar(&opts.webhookPort, "webhook-port", opts.webhookPort,
		"Webhook Server port")
	cmd.Flags().StringVar(&opts.webhookCertDir, "webhook-cert-dir", opts.webhookCertDir,
		"Webhook cert dir, only used when webhook-port is specified.")
	cmd.Flags().StringVar(&opts.healthAddr, "health-addr", opts.healthAddr,
		"The address the health endpoint binds to.")
	cmd.Flags().StringVar(&opts.KubeconfigPath, "kubeconfig", opts.KubeconfigPath, "")

	zapFs := flag.NewFlagSet("", flag.ExitOnError)
	klog.InitFlags(nil)
	opts.zapOpts.BindFlags(zapFs)
	cmd.Flags().AddGoFlagSet(zapFs)

	return cmd
}

func (o *RunOptions) Complete(cmd *cobra.Command, args []string) error {
	return o.RootOptions.Complete(cmd, args)
}

func (o *RunOptions) Validate() error {
	return o.RootOptions.Validate()
}

func (o *RunOptions) Run(ctx context.Context) error {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&o.zapOpts)))
	log := ctrl.Log.WithName("setup")

	if o.profilerAddress != "" {
		log.Info("Profiler listening for requests", "address", o.profilerAddress)
		go func() {
			err := http.ListenAndServe(o.profilerAddress, nil)
			if err != nil {
				log.Error(err, "profiler exited")
			}
		}()
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", o.KubeconfigPath)
	if err != nil {
		return err
	}
	restConfig.UserAgent = remote.DefaultClusterAPIUserAgent("cluster-api-provider-external")

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(externalinfrav1.AddToScheme(scheme))
	utilruntime.Must(externalcontrolplanev1.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     o.metricsBindAddr,
		LeaderElection:         o.enableLeaderElection,
		LeaderElectionID:       "cluster-api-provider-external-leader-election-capi",
		LeaseDuration:          &o.leaderElectionLeaseDuration,
		RenewDeadline:          &o.leaderElectionRenewDeadline,
		RetryPeriod:            &o.leaderElectionRetryPeriod,
		Namespace:              o.watchNamespace,
		SyncPeriod:             &o.syncPeriod,
		Port:                   o.webhookPort,
		HealthProbeBindAddress: o.healthAddr,
		CertDir:                o.webhookCertDir,
	})
	if err != nil {
		return err
	}

	if err = (&controllers.ExternalClusterReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create controller %s: %w", "ExternalCluster", err)
	}
	log.Info("Started ExternalCluster reconciler")

	if err = (&controllers.ExternalControlPlaneReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create controller %s: %w", "ExternalControlPlane", err)
	}
	log.Info("Started ExternalControlPlane reconciler")

	if err = (&controllers.ExternalMachineReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create controller %s: %w", "ExternalMachine", err)
	}
	log.Info("Started ExternalMachine reconciler")

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	// +kubebuilder:scaffold:builder
	log.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("problem running manager: %w", err)
	}
	return nil
}
