---
name: local-dev-troubleshooting
description: "Common failures in local Gardener dev environment and their fixes — network policies, disk pressure, stuck machines, extension connectivity"
metadata: 
  node_type: memory
  type: reference
  originSessionId: 1dabdd3f-b2ee-4599-af9d-c4de6be5ce1e
---

## Common Issues and Fixes

### Extension pods can't reach virtual garden API (i/o timeout)

**Symptom**: Extension pods in `extension-rtsecurity/extension-extension-shoot-rtsecurity-service-*` namespace crash with connection timeout to the virtual garden API (172.18.x.x LoadBalancer IP).

**Root cause**: In local kind setup, the virtual garden API is exposed via a LoadBalancer IP in the private range (172.18.0.0/12). The extension pods need the network policy label `networking.gardener.cloud/to-private-networks: allowed`.

**Fix**: Already applied in `charts/gardener-extension-shoot-rtsecurity-service/templates/deployment.yaml` — the pod template includes:
```yaml
networking.gardener.cloud/to-private-networks: allowed
networking.resources.gardener.cloud/to-virtual-garden-istio-ingress-istio-ingressgateway-inte-02e09: allowed
networking.resources.gardener.cloud/to-all-shoots-kube-apiserver-tcp-443: allowed
```

### Disk pressure on kind node

**Symptom**: Worker machines get `DiskPressure` condition, machines stuck in `Terminating`, shoot reconcile fails with "machine deployments not ready".

**Fix**:
```bash
docker system prune -f
docker image prune -a -f
docker exec gardener-local-control-plane crictl rmi --prune
```

Check with: `docker exec gardener-local-control-plane df -h /`
Target: below 80% usage.

### Machine stuck in Terminating

**Symptom**: `kubectl -n shoot--local--soc-test get machines` shows machine in `Terminating` state indefinitely.

**Fix** (if drain is stuck and won't resolve):
```bash
KUBECONFIG=$KUBECONFIG_RUNTIME kubectl -n shoot--local--soc-test patch machine <name> --type=merge -p '{"metadata":{"finalizers":null}}'
```

If the shoot is completely broken, it's often faster to delete and recreate:
```bash
KUBECONFIG=$KUBECONFIG_VIRTUAL kubectl -n garden-local annotate shoot soc-test "confirmation.gardener.cloud/deletion=true" --overwrite
KUBECONFIG=$KUBECONFIG_VIRTUAL kubectl -n garden-local delete shoot soc-test
```

### Reconcile fails: "secretRef not found in resources"

The extension passes the pixie credentials and both registry pull secrets by reference
rather than inline, because providerConfig is part of the Shoot spec and readable by
anyone who can read the Shoot. Each ref needs its spec.resources stanza or the reconcile
fails partway through.

**Symptom**: Shoot creation fails at ~51% with error "could not generate SOC configuration: resource "soc-pull-entlein" is declared but ref-soc-pull-entlein is not readable".

**Root cause**: the Shoot does not declare that credential in `spec.resources`, so gardener never mirrors it into the shoot namespace as `ref-<name>`.

**Fix**: Add to the Shoot spec:
```yaml
spec:
  resources:
  - name: opensearch-config
    resourceRef:
      apiVersion: v1
      kind: Secret
      name: opensearch-config
```

