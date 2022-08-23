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

// MachineScopeParams defines the input parameters used to create a new Scope.
type MachineScopeParams struct {
	Client          client.Client
	Logger          logr.Logger
	Cluster         *clusterv1beta1.Cluster
	Machine         *clusterv1beta1.Machine
	ExternalMachine *externalv1.ExternalMachine
}

// NewMachineScope creates a new MachineScope from the supplied parameters.
// This is meant to be called for each reconcile iteration only on MachineReconciler.
func NewMachineScope(params MachineScopeParams) (*ExternalMachineScope, error) {
	if params.Machine == nil {
		return nil, errors.New("Machine is required when creating a ExternalMachineScope")
	}
	if params.ExternalMachine == nil {
		return nil, errors.New("ExternalMachine is required when creating a ExternalMachineScope")
	}
	// if params.Logger == nil {
	// 	params.Logger = klogr.New()
	// }

	helper, err := patch.NewHelper(params.ExternalMachine, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ExternalMachineScope{
		Logger:          params.Logger,
		client:          params.Client,
		Machine:         params.Machine,
		ExternalMachine: params.ExternalMachine,
		patchHelper:     helper,
	}, nil
}

// MachineScope defines the basic context for an actuator to operate upon.
type ExternalMachineScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	Cluster         *clusterv1beta1.Cluster
	Machine         *clusterv1beta1.Machine
	ExternalMachine *externalv1.ExternalMachine
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ExternalMachineScope) Close() error {
	return s.patchHelper.Patch(context.TODO(), s.ExternalMachine)
}

// Name returns the cluster name.
func (s *ExternalMachineScope) Name() string {
	return s.Machine.GetName()
}

// Namespace returns the cluster namespace.
func (s *ExternalMachineScope) Namespace() string {
	return s.Machine.GetNamespace()
}

func (s *ExternalMachineScope) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: s.Machine.Namespace,
		Name:      s.Machine.Name,
	}
}

// SetReady sets the ExternalMachine Ready Status
func (s *ExternalMachineScope) SetReady() {
	s.ExternalMachine.Status.Ready = true
}
