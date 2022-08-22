# Cluster API provider 

## Usage

### 1a. Import an external cluster manually


### 1b. Import a cluster using the CLI

```bash
clusterctl-external import $KUBECONFIG
```

## Roadmap

- [ ] Properly report the status of the cluster.
- [ ] Add webhooks to block unsupported operations (e.g., delete machine).
- [ ] Determine version of cluster.
- [ ] Support agent-based external clusters.
- [ ] Check and warn if the imported cluster won't be accessible from the management cluster.