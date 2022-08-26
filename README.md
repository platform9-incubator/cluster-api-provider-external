# Cluster API Provider External

The Kubernetes Cluster API Provider External (CAPE) enables declarative importing of arbitrary Kubernetes clusters on any infrastructure. This enables bringing in externally-managed clusters into your Cluster API control plane, making it a single pane of glass for your clusters. Although it is not possible to handle all the (infrastructure-related) operations for these external clusters, it allows for access management and orchestrating higher-level operations (such as installing addons or integrating them with GitOps tooling) on these clusters.

## Installation

To deploy CAPE in your cluster:
```bash
make deploy
```

To install the CLI on your system
```bash
go install -o cape .
```

## Usage

### 1a. Import an external cluster manually

TBD

### 1b. Import a cluster using the CLI

```bash
cape import --mgmt-kubeconfig $SUNPIKE_KUBECONFIG --kubeconfig $KUBECONFIG --name example-imported-cluster
```
