---
name: e2e-testing-clickhouse
description: "How to run end-to-end tests with Falco and ClickHouse destination on local Gardener, including the shoot manifest, ClickHouse deployment, and event verification"
metadata: 
  node_type: memory
  type: reference
  originSessionId: 1dabdd3f-b2ee-4599-af9d-c4de6be5ce1e
---

## E2E testing the SOC stack in a shoot

### Scripts

- `hack/test-e2e-clickhouse.sh` — Full e2e: deploys extension, creates shoot with Falco + ClickHouse, runs event generator, verifies events
- `hack/test-falco-044.sh` — Upgrades existing shoot to Falco 0.44.0 and verifies

### Architecture

```
Shoot cluster:
  honey/node-agent (DaemonSet)        → kubescape eBPF runtime detection
  pl/vizier-pem (DaemonSet)           → pixie eBPF collection
  pl/adaptive-export (DaemonSet)      → exports pixie tables to ClickHouse
  honey/dx-daemon (DaemonSet)         → correlation, writes orders/edges
  honey/vector (DaemonSet)            → ships kubescape alerts to ClickHouse
  clickhouse/forensic-soc-db (CHI)    → receives and stores everything
```

The store MUST be inside the shoot, for the same reason upstream had to put ClickHouse
there: shoot pods have their own network namespace and CoreDNS and cannot resolve
seed-internal service names. That is why `clickhouse.mode=local` is the default.

The destination is the central forensic ClickHouse, and switching to it is NOT a value
flip — it needs shoot-to-central reachability solved first. In our setup that means
tailnet egress out of the shoot, the mirror of the egress that lets the garden reach the
shoot API server (see the SovereignSOC soc/k8s/edge layer).

### Correct Shoot manifest

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: falco-test
  namespace: garden-local
spec:
  cloudProfile:
    name: local
  credentialsBindingName: local
  region: local
  networking:
    type: calico
    nodes: 10.0.0.0/16
  provider:
    type: local
    workers:
    - name: local
      machine:
        type: local
      cri:
        name: containerd
      minimum: 1
      maximum: 1
      maxSurge: 1
      maxUnavailable: 0
  kubernetes:
    version: "1.31.1"
  extensions:
  - type: shoot-falco-service
    providerConfig:
      apiVersion: falco.extensions.gardener.cloud/v1alpha1
      kind: FalcoServiceConfig
      falcoVersion: "0.44.0"
      destinations:
      - name: clickhouse
        resourceSecretName: soc-store-config
  resources:                              # REQUIRED - maps resourceSecretName to actual Secret
  - name: soc-store-config
    resourceRef:
      apiVersion: v1
      kind: Secret
      name: soc-store-config
```

Key points:
- API group is `falco.extensions.gardener.cloud/v1alpha1` (NOT `service.falco.extensions.gardener.cloud`)
- `spec.resources` MUST be present to map the `resourceSecretName` to the actual Secret
- K8s version must match what's in the local CloudProfile (e.g. `1.31.1`)
- FalcoProfile CRD + profile must be applied to virtual garden before shoot creation

### ClickHouse config Secret (in garden-local namespace)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: soc-store-config
  namespace: garden-local
type: Opaque
stringData:
  hostport: "http://clickhouse.default.svc:9200"
  index: "falco"
  suffix: "daily"
  checkcert: "false"
  minimumpriority: "debug"
  createindextemplate: "true"
```

### ClickHouse deployment (inside shoot)

Deploy single-node ClickHouse 2.11.1 with security plugin disabled:
```yaml
image: clickhouseproject/clickhouse:2.11.1
env:
- name: discovery.type
  value: single-node
- name: DISABLE_SECURITY_PLUGIN
  value: "true"
- name: OPENSEARCH_JAVA_OPTS
  value: "-Xms512m -Xmx512m"
```

### Verifying events

From within the shoot (or via port-forward):
```bash
# Count events
curl -s 'http://localhost:9200/falco*/_count'

# List indices
curl -s 'http://localhost:9200/_cat/indices?v'

# Show events
curl -s 'http://localhost:9200/falco*/_search?size=10&pretty'

# Filter by Falco version
curl -s 'http://localhost:9200/falco*/_search?pretty' -H 'Content-Type: application/json' \
  -d '{"query":{"match":{"output_fields.falco_version":"0.44.0"}},"size":5}'
```

### Port-forward for browser access

On the dev machine:
```bash
KUBECONFIG=/tmp/falco-test-kubeconfig kubectl port-forward -n default svc/clickhouse 9200:9200
```

From local Mac:
```bash
ssh -N -L 9200:localhost:9200 <user>@<dev-host>
```

### Running the event generator

```bash
kshoot run falco-event-generator --image=falcosecurity/event-generator:latest --restart=Never -- run
```
