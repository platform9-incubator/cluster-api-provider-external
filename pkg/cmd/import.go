package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/erwinvaneyk/cobras"
	externalcontrolplanev1 "github.com/platform9/cluster-api-provider-external/api/controlplane/v1beta1"
	externalinfrav1 "github.com/platform9/cluster-api-provider-external/api/infrastructure/v1beta1"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigOptions struct {
	*RootOptions
	MgmtKubeconfigPath    string
	MgmtClusterNamespace  string
	ClusterName           string
	ClusterKubeconfigPath string
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

	return cmd
}

func (o *ConfigOptions) Complete(cmd *cobra.Command, args []string) error {
	return o.RootOptions.Complete(cmd, args)
}

func (o *ConfigOptions) Validate() error {
	if len(o.ClusterName) == 0 {
		return errors.New("name of the target cluster is required")
	}
	if len(o.MgmtKubeconfigPath) == 0 {
		return errors.New("kubeconfig for the management cluster is required")
	}
	if len(o.ClusterKubeconfigPath) == 0 {
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

	log.Debugf("Creating an ExternalCluster for cluster")
	resources := []client.Object{
		&clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.ClusterName,
				Namespace: o.MgmtClusterNamespace,
			},
			Spec: clusterv1.ClusterSpec{
				ControlPlaneRef: &corev1.ObjectReference{
					APIVersion: externalcontrolplanev1.GroupVersion.String(),
					Kind:       "ExternalControlPlane",
					Name:       o.ClusterName,
				},
				InfrastructureRef: &corev1.ObjectReference{
					APIVersion: externalinfrav1.GroupVersion.String(),
					Kind:       "ExternalCluster",
					Name:       o.ClusterName,
				},
			},
		},
		&externalinfrav1.ExternalCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.ClusterName,
				Namespace: o.MgmtClusterNamespace,
			},
			Spec: externalinfrav1.ExternalClusterSpec{
				ControlPlaneEndpoint: clusterv1.APIEndpoint{
					Host: host,
					Port: int32(port),
				},
			},
		},
		&externalcontrolplanev1.ExternalControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.ClusterName,
				Namespace: o.MgmtClusterNamespace,
			},
			Spec: externalcontrolplanev1.ExternalControlPlaneSpec{},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-kubeconfig", o.ClusterName),
				Namespace: o.MgmtClusterNamespace,
			},
			Immutable: pointer.Bool(true),
			StringData: map[string]string{
				"value": string(bs),
			},
			Type: clusterv1.ClusterSecretType,
		},
	}
	for _, resource := range resources {
		log.Debugf("Creating resource %T: %s/%s", resource, resource.GetNamespace(), resource.GetName())
		err := mgmtClient.Create(ctx, resource)
		if err != nil {
			return err
		}
	}

	fmt.Printf("cluster imported as %s/%s.\n", o.MgmtClusterNamespace, o.ClusterName)
	return nil
}
