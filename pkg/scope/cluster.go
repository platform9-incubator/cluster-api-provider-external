/*
Copyright 2020 The Kubernetes Authors.

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

package scope

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	externalv1 "github.com/platform9-incubator/cluster-api-provider-external/api/infrastructure/v1beta1"
	"k8s.io/apimachinery/pkg/types"

	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterScopeParams defines the input parameters used to create a new Scope.
type ClusterScopeParams struct {
	Client          client.Client
	Logger          logr.Logger
	Cluster         *clusterv1beta1.Cluster
	ExternalCluster *externalv1.ExternalCluster
}

// NewClusterScope creates a new ClusterScope from the supplied parameters.
// This is meant to be called for each reconcile iteration only on ClusterReconciler.
func NewClusterScope(params ClusterScopeParams) (*ExternalClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("Cluster is required when creating a ExternalClusterScope")
	}
	if params.ExternalCluster == nil {
		return nil, errors.New("ExternalCluster is required when creating a ExternalClusterScope")
	}
	// if params.Logger == nil {
	// 	params.Logger = klogr.New()
	// }

	helper, err := patch.NewHelper(params.ExternalCluster, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ExternalClusterScope{
		Logger:          params.Logger,
		client:          params.Client,
		Cluster:         params.Cluster,
		ExternalCluster: params.ExternalCluster,
		patchHelper:     helper,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ExternalClusterScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	Cluster         *clusterv1beta1.Cluster
	ExternalCluster *externalv1.ExternalCluster
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ExternalClusterScope) Close() error {
	return s.patchHelper.Patch(context.TODO(), s.ExternalCluster)
}

// Name returns the cluster name.
func (s *ExternalClusterScope) Name() string {
	return s.Cluster.GetName()
}

// Namespace returns the cluster namespace.
func (s *ExternalClusterScope) Namespace() string {
	return s.Cluster.GetNamespace()
}

func (s *ExternalClusterScope) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: s.Cluster.Namespace,
		Name:      s.Cluster.Name,
	}
}

// SetReady sets the ExternalCluster Ready Status
func (s *ExternalClusterScope) SetReady() {
	s.ExternalCluster.Status.Ready = true
}
