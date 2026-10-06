---
name: soc-chart-port
description: "Where each imperative deploy step goes now that the SOC stack is an extension chart, and which ones disappear"
metadata:
  node_type: memory
  type: reference
---

## Porting the stack from deploy scripts into the extension chart

An extension renders a chart into a ManagedResource and GRM applies it. It cannot run
kubectl, read a file from the operator's machine, or exec into a pod. So every imperative
deploy step has to go somewhere — and most land somewhere better, because they only
existed because a person's machine was doing the deploying.

| the deploy script did | becomes |
|---|---|
| patch the `adaptive-export-control` Service selector | the chart owns that Service, so GRM corrects drift continuously where the patch ran once |
| post-deploy check that the Service has endpoints | the healthcheck controller: continuous, not deploy-time |
| `kubectl rollout restart` the node-agent after a rules change | a checksum annotation on the pod template |
| append the port to the cloud address in the operator's ConfigMap | the component is given `cloudAddr` as a value; the patch existed only because it inherited a ConfigMap it did not own |
| build pull secrets from local docker configs | gardener NamedResourceReferences, resolved by the actuator |
| copy a signing key out of the vizier's cluster secret | the consumer runs in the same namespace and mounts it directly |
| repair ClickHouse schema drift before deploying | not the deployer's job; the writer owns its migrations |

## Pixie without the px binary

`px deploy`'s real output is a cloud ConfigMap and a deploy-key Secret, created as side
effects. Those are just objects, and pixie ships declarative kustomizations for the
operator and the Vizier CR, so the chart renders them and the binary never goes near the
shoot. px stays a human tool used from outside.

Two values that are not optional:
- `clusterName` is derived from the Shoot name by `pkg/values`, not configured. A cluster
  left at the operator default registers a new `default_<hash>` in the pixie cloud on
  every reinstall instead of reclaiming its own. (Shoot.Status.TechnicalID is the
  collision-safe variant if two projects ever reuse a shoot name; the plain name is used
  because humans read it in the UI.)
- `cloudAddr` must carry an explicit port or the cloud clients crash-loop.

## ClickHouse placement

`clickhouse.mode=local` — one per shoot. Not a preference: shoot pods have their own
network namespace and DNS and cannot resolve seed-internal services. A central ClickHouse
is the destination, and getting there needs shoot-to-central reachability first.
