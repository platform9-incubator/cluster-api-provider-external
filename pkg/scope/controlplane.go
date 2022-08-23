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
	externalv1 "github.com/platform9-incubator/cluster-api-provider-external/api/controlplane/v1beta1"

	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ControlPlaneScopeParams defines the input parameters used to create a new Scope.
type ControlPlaneScopeParams struct {
	Client               client.Client
	Logger               logr.Logger
	Cluster              *clusterv1beta1.Cluster
	ExternalControlPlane *externalv1.ExternalControlPlane
}

// NewControlPlaneScope creates a new ControlPlaneScope from the supplied parameters.
// This is meant to be called for each reconcile iteration only on ClusterReconciler.
func NewControlPlaneScope(params ControlPlaneScopeParams) (*ControlPlaneScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("Cluster is required when creating a ControlPlaneScope")
	}
	if params.ExternalControlPlane == nil {
		return nil, errors.New("ExternalControlPlane is required when creating a ControlPlaneScope")
	}
	// if params.Logger == nil {
	// 	params.Logger = klogr.New()
	// }

	helper, err := patch.NewHelper(params.ExternalControlPlane, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ControlPlaneScope{
		Logger:               params.Logger,
		client:               params.Client,
		Cluster:              params.Cluster,
		ExternalControlPlane: params.ExternalControlPlane,
		patchHelper:          helper,
	}, nil
}

// ControlPlaneScope defines the basic context for an actuator to operate upon.
type ControlPlaneScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	Cluster              *clusterv1beta1.Cluster
	ExternalControlPlane *externalv1.ExternalControlPlane
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ControlPlaneScope) Close() error {
	return s.patchHelper.Patch(context.TODO(), s.ExternalControlPlane)
}

// Name returns the cluster name.
func (s *ControlPlaneScope) Name() string {
	return s.Cluster.GetName()
}

// Namespace returns the cluster namespace.
func (s *ControlPlaneScope) Namespace() string {
	return s.Cluster.GetNamespace()
}

// SetReady sets the ExternalControlPlane Ready Status
func (s *ControlPlaneScope) SetReady() {
	s.ExternalControlPlane.Status.Ready = true
}
