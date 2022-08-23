package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/erwinvaneyk/cobras"
	externalcontrolplanev1 "github.com/platform9-incubator/cluster-api-provider-external/api/controlplane/v1beta1"
	externalinfrav1 "github.com/platform9-incubator/cluster-api-provider-external/api/infrastructure/v1beta1"
	importer "github.com/platform9-incubator/cluster-api-provider-external/pkg/cape"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigOptions struct {
	*RootOptions
	MgmtKubeconfigPath    string
	MgmtClusterNamespace  string
	ClusterName           string
	ClusterKubeconfigPath string
	ImportFromQbert       bool
	Username              string
	Password              string
	Project               string
	FQDN                  string
}

func NewCmdImport(rootOptions *RootOptions) *cobra.Command {
	opts := &ConfigOptions{
		RootOptions:          rootOptions,
		MgmtClusterNamespace: metav1.NamespaceDefault,
	}

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import an external cluster into CAPI.",
		Run:   cobras.Run(opts),
	}

	cmd.Flags().StringVarP(&opts.MgmtClusterNamespace, "namespace", "n", opts.MgmtClusterNamespace, "Namespace to create the cloud provider in.")
	cmd.Flags().StringVar(&opts.ClusterKubeconfigPath, "kubeconfig", opts.ClusterKubeconfigPath, "Kubeconfig of the cluster to import.")
	cmd.Flags().StringVar(&opts.MgmtKubeconfigPath, "mgmt-kubeconfig", opts.MgmtKubeconfigPath, "Kubeconfig of the management cluster to import the cluster into.")
	cmd.Flags().StringVar(&opts.ClusterName, "name", opts.ClusterName, "Name of the cluster to import.")
	cmd.Flags().BoolVar(&opts.ImportFromQbert, "qbert", false, "import all clusters from qbert")
	cmd.Flags().StringVar(&opts.Username, "username", "", "username to connect to the PF9 control plane")
	cmd.Flags().StringVar(&opts.Password, "password", "", "password to connect to the PF9 control plane")
	cmd.Flags().StringVar(&opts.Project, "project", "service", "project to authenticate as when connecting to the PF9 control plane")
	cmd.Flags().StringVar(&opts.FQDN, "fqdn", "", "PF9 control plane URL")

	return cmd
}

func (o *ConfigOptions) Complete(cmd *cobra.Command, args []string) error {
	return o.RootOptions.Complete(cmd, args)
}

func (o *ConfigOptions) Validate() error {
	if len(o.ClusterName) == 0 && !o.ImportFromQbert {
		return errors.New("name of the target cluster is required")
	}
	if len(o.MgmtKubeconfigPath) == 0 {
		return errors.New("kubeconfig for the management cluster is required")
	}
	if len(o.ClusterKubeconfigPath) == 0 && !o.ImportFromQbert {
		return errors.New("kubeconfig for the target cluster is required")
	}
	return o.RootOptions.Validate()
}

func (o *ConfigOptions) Run(ctx context.Context) error {
	log := zap.S()

	log.Debugf("Setting up mgmt cluster client")
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(externalinfrav1.AddToScheme(scheme))
	utilruntime.Must(externalcontrolplanev1.AddToScheme(scheme))

	cfg, err := clientcmd.BuildConfigFromFlags("", o.MgmtKubeconfigPath)
	if err != nil {
		return err
	}
	mgmtClient, err := client.New(cfg, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return err
	}

	log.Debugf("Loading workload cluster client")
	workloadCfg, err := clientcmd.BuildConfigFromFlags("", o.ClusterKubeconfigPath)
	if err != nil {
		return err
	}

	hostParts := strings.Split(strings.TrimPrefix(workloadCfg.Host, "https://"), ":")
	port := 6443
	host := hostParts[0]
	if len(hostParts) > 2 {
		port, err = strconv.Atoi(hostParts[1])
		if err != nil {
			return err
		}
	}
	bs, err := ioutil.ReadFile(o.ClusterKubeconfigPath)
	if err != nil {
		return err
	}

	clsImporter := importer.ClusterImporter{
		MgmtClient: mgmtClient,
		Log:        log,
	}

	if o.ImportFromQbert {
		if o.MgmtClusterNamespace == "" {
			o.MgmtClusterNamespace = "default"
		}
		return clsImporter.ImportClustersFromQbert(ctx, o.Username, o.Password, o.Project, "RegionOne", o.MgmtClusterNamespace, o.FQDN)
	}
	log.Debugf("Creating an ExternalCluster for cluster")
	err = clsImporter.ImportClusterResources(ctx, o.ClusterName, o.MgmtClusterNamespace, host, port, string(bs))
	if err != nil {
		panic(fmt.Sprintf("cluster import failed: %v", err))
	}

	fmt.Printf("cluster imported as %s/%s.\n", o.MgmtClusterNamespace, o.ClusterName)
	return nil
}
