package webhooks

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type MachineWebhook struct {
	Client  client.Client
	decoder *admission.Decoder
}

var _ admission.Handler = (*MachineWebhook)(nil)

func (n *MachineWebhook) InjectDecoder(d *admission.Decoder) error {
	n.decoder = d
	return nil
}

func (n *MachineWebhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	machine := &clusterv1.Machine{}
	err := n.decoder.Decode(req, machine)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Only handle machines that are external machines
	if machine.Spec.InfrastructureRef.Kind != "ExternalMachine" {
		return admission.Allowed("")
	}

	// Only handle delete requests
	if req.Operation != admissionv1.Delete {
		return admission.Allowed("")
	}

	// Check if the cluster is being deleted
	cluster, err := util.GetClusterFromMetadata(ctx, n.Client, machine.ObjectMeta)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Only allow deletion when the cluster is being deleted. This is needed
	// because in the CAPI core logic, machines being deleted separately from
	// the cluster will also cause the node to be drained and deleted.
	// TODO upstream a patch to skip node preemption/deletion when deleting a machine.
	if cluster.DeletionTimestamp == nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("external machines are read-only"))
	}

	return admission.Allowed("")
}
