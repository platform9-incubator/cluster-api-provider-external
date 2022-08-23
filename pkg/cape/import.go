package cape

import (
	"context"
	"fmt"
	"strconv"

	externalcontrolplanev1 "github.com/platform9-incubator/cluster-api-provider-external/api/controlplane/v1beta1"
	externalinfrav1 "github.com/platform9-incubator/cluster-api-provider-external/api/infrastructure/v1beta1"
	"github.com/platform9/pf9-sdk-go/pf9/du"
	"github.com/platform9/pf9-sdk-go/pf9/keystone"
	"github.com/platform9/pf9-sdk-go/pf9/qbert"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterImporter struct {
	MgmtClient client.Client
	Log        *zap.SugaredLogger
}

func (c *ClusterImporter) ImportClusterResources(ctx context.Context, ClusterName string, MgmtClusterNamespace string, host string, port int, workloadClusterKubeconfig string) error {
	resources := []client.Object{
		&clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ClusterName,
				Namespace: MgmtClusterNamespace,
			},
			Spec: clusterv1.ClusterSpec{
				ControlPlaneRef: &corev1.ObjectReference{
					APIVersion: externalcontrolplanev1.GroupVersion.String(),
					Kind:       "ExternalControlPlane",
					Name:       ClusterName,
				},
				InfrastructureRef: &corev1.ObjectReference{
					APIVersion: externalinfrav1.GroupVersion.String(),
					Kind:       "ExternalCluster",
					Name:       ClusterName,
				},
			},
		},
		&externalinfrav1.ExternalCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ClusterName,
				Namespace: MgmtClusterNamespace,
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
				Name:      ClusterName,
				Namespace: MgmtClusterNamespace,
			},
			Spec: externalcontrolplanev1.ExternalControlPlaneSpec{},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-kubeconfig", ClusterName),
				Namespace: MgmtClusterNamespace,
			},
			Immutable: pointer.Bool(true),
			StringData: map[string]string{
				"value": workloadClusterKubeconfig,
			},
			Type: clusterv1.ClusterSecretType,
		},
	}
	for _, resource := range resources {
		c.Log.Debugf("Creating resource %T: %s/%s", resource, resource.GetNamespace(), resource.GetName())
		err := c.MgmtClient.Create(ctx, resource)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *ClusterImporter) ImportClustersFromQbert(ctx context.Context, username string, password string, project string, region string, managementClusterNamespace string, fqdn string) error {
	keystoneEndpoint := fmt.Sprintf("%s/keystone", fqdn)
	creds := keystone.Credentials{
		Username: username,
		Password: password,
		Tenant:   project,
		Region:   region,
	}
	ksClient := keystone.NewClient(keystoneEndpoint)
	basicAuth := keystone.NewBasicTokenGenerator(ksClient, creds)
	qbertClient := qbert.NewQbert(qbert.Config{
		DU: du.Info{
			FQDN: fqdn,
		},
		Authenticator: basicAuth,
	})
	auth, err := basicAuth.Auth(ctx)
	if err != nil {
		panic(fmt.Sprintf("could not authenticate: %v", err))
	}
	qbertClusters, err := qbertClient.ListClusters()
	if err != nil {
		panic(fmt.Sprintf("could not list qbert clusters: %v", err))
	}
	for _, cluster := range qbertClusters {
		// TODO: fix token 0-> base64 encoded username/password string
		clusterKubeConfig, err := qbertClient.GetClusterKubeconfig(cluster.ProjectID, cluster.UUID, auth.Token)
		if err != nil {
			c.Log.Debugf("could not fetch kubeconfig for %s cluster. Not registering it.", cluster.Name)
		}
		apiPort, _ := strconv.Atoi(cluster.APIPort)
		err = c.ImportClusterResources(ctx, cluster.Name, managementClusterNamespace, cluster.ExternalDNSName, apiPort, string(clusterKubeConfig))
		if err != nil {
			c.Log.Debugf("failed to register %s cluster: %v", cluster.Name, err)
		}
	}
	return nil
}
