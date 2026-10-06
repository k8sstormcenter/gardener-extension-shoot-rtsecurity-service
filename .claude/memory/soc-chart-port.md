---
name: soc-chart-port
description: "Where each skaffold hook goes when the SOC stack becomes an extension chart, and which ones disappear"
metadata:
  node_type: memory
  type: reference
---

## Porting the SOC skaffolds into the extension chart

An extension renders a chart into a ManagedResource and GRM applies it. It cannot run
kubectl, read `$HOME`, or exec into a pod. So every imperative skaffold hook has to go
somewhere — and most land somewhere better, because they existed only because a laptop
was doing the deploying.

Sources: soc `skaffold.yaml` (modules soc-clickhouse, soc-kubescape, soc-vector,
soc-stack) and pixie `skaffold/skaffold_adaptive_export.yaml`, `skaffold_dx.yaml`.

| skaffold did | becomes |
|---|---|
| patch `adaptive-export-control` Service selector to `{"name":"adaptive-export"}` | the chart OWNS that Service. GRM then corrects drift continuously, where the hook ran once |
| after-hook: fail if `adaptive-export-control` has no endpoints | the extension's healthcheck controller. Continuous, not deploy-time |
| `kubectl rollout restart ds/node-agent` after rules change | checksum annotation on the pod template |
| patch `pl-cloud-config` to append `:443` to `PL_CLOUD_ADDR` | AE gets `cloudAddr` as a value. The patch existed only because AE inherited a ConfigMap it did not own |
| pull secrets from `$HOME/.docker/config.json` and `~/.registry-pull/tanzeee/config.json` | gardener NamedResourceReferences, resolved by the actuator |
| dx hook extracting a JWT from `pl-cluster-secrets` | dx mounts that secret directly. Same cluster, so copying it was never needed |
| ClickHouse schema-drift check (drop AE objects when `conn_stats` lacks `unique_id`) | NOT the deployer's job. AE owns its migrations (`internal/ae/clickhouse/migrate.go`, `addcolumns.go`) |

## Pixie without the px binary

`px deploy` is a laptop workflow whose real output is `pl-cloud-config` and
`pl-deploy-secrets` as side effects. Those are just objects. The pixie fork already has
declarative paths — `k8s/operator/deployment/sbob` and `k8s/vizier/sbob` kustomizations —
so the extension renders the operator plus the Vizier CR and the px binary never goes
near the shoot. px stays a human tool for `px auth` / `px run` from outside.

Two values that are not optional:
- `clusterName` is derived from the Shoot name by `pkg/values`, not configured. The
  actuator already reads it off the Cluster resource. Deriving it is what stops
  registration sprawl at source — a cluster left at the operator default makes the pixie
  cloud accumulated six `default_<hash>` registrations, five dead, because each reinstall
  registered a new one. A derived name is stable across reinstalls of the same shoot.
  (Shoot.Status.TechnicalID is the collision-safe variant if two projects ever reuse a
  shoot name; the plain name is used because humans read it in the UI.)
- `cloudAddr` must carry `:443` or the AE cloud client crash-loops.

## ClickHouse placement

`clickhouse.mode=local` — one per shoot. Not a preference: shoot pods have their own
network namespace and CoreDNS and cannot resolve seed-internal services, which is the
same wall upstream hit with OpenSearch. The central forensic ClickHouse is the
destination, and getting there needs shoot-to-central reachability first.
