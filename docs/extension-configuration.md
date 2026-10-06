# Configuration

Two separate things are configured: the extension controller (by the operator, once) and
each shoot's stack (by whoever writes the Shoot).

## Extension controller: `--config-file`

```yaml
apiVersion: rtsecurity.extensions.config.gardener.cloud/v1alpha1
kind: Configuration
healthCheckConfig:
  syncPeriod: 30s
```

`healthCheckConfig` is gardener's standard health-check controller config. Everything else
about the deployed stack is per-shoot or a chart default — there is deliberately no
operator-level copy of a value the chart already owns.

## Per shoot: `providerConfig`

Every field is optional; unset means the chart default in
`charts/internal/soc/values.yaml`. Unknown fields and bad enum values fail the reconcile
rather than being ignored.

```yaml
spec:
  extensions:
    - type: shoot-rtsecurity-service
      providerConfig:
        apiVersion: rtsecurity.extensions.gardener.cloud/v1alpha1
        kind: SOCConfig
        mode: selfcontained          # selfcontained: ClickHouse in the shoot
                                     # full: central ClickHouse (needs clickhouse.centralHost)
        pixie:
          cloudAddr: work.example.com:443   # must carry an explicit port
          vizierVersion: 0.14.19-pemdq1
          dataAccess: Full
          pemMemoryLimit: 4Gi
          pemMemoryRequest: 2Gi
        clickhouse:
          retentionHours: 24
          storage: 5Gi
          centralHost: ""
        detection:
          mode: default              # default | alert | enforce
          rules: []                  # bound rule names; empty = the SOC default set
          excludeNamespaces: []      # sensor ignore list
          bindingExcludeNamespaces: []
        components:
          kubescape: true
          vector: false
          adaptiveExport: true
          dx: true
        credentials:                 # names of spec.resources[] entries, see below
          pixieDeployKey: ""
          pixieApiKey: ""
          pullEntlein: ""
          pullTanzeee: ""
        images: {}                   # overrides keyed like values.images
```

`detection.mode` is written as `.policy.mode` of the node-agent's trust policy: unset
behaves as `alert`, `enforce` refuses unsigned or unverifiable artifacts.

The pixie cluster name is **not** configurable: it is the Shoot's own name. A derived name
cannot be forgotten, and a forgotten one registers a new cluster in the pixie cloud on
every reinstall.

## Credentials

The stack needs a pixie deploy key, a pixie API key and two registry pull secrets. They
are referenced, never inlined — `providerConfig` is part of the Shoot and readable by
anyone who can read the Shoot:

```yaml
spec:
  resources:
    - name: soc-pixie-deploy-key
      resourceRef:
        apiVersion: v1
        kind: Secret
        name: soc-pixie-deploy-key
    - name: soc-pixie-api-key
      ...
    - name: soc-pull-entlein
      ...        # kubernetes.io/dockerconfigjson
    - name: soc-pull-tanzeee
      ...        # kubernetes.io/dockerconfigjson
```

Gardener mirrors each referenced Secret into the shoot namespace as `ref-<name>`, which is
where the actuator reads it. The four names above are the defaults; override them in
`providerConfig.credentials`. A missing reference renders the stack without that
credential, except the API key, which adaptive-export and dx cannot run without.
