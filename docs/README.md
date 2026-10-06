# shoot-rtsecurity-service

A Gardener extension that deploys the SOC runtime-security stack into a shoot: pixie
(operator + Vizier + PEMs), kubescape's node-agent with the SOC rule set,
adaptive-export, dx-daemon, and a ClickHouse to hold the evidence.

The stack is rendered from one embedded chart (`charts/internal/soc`) into a single
`ManagedResource`, so gardener-resource-manager owns every object in the shoot and the
extension itself runs only on the seed.

- [Extension configuration](extension-configuration.md) — the operator's `--config-file`
  and the per-shoot `providerConfig`.

Enable it on a shoot:

```yaml
spec:
  extensions:
    - type: shoot-rtsecurity-service
```

and declare the credentials it needs in `spec.resources` (see the configuration doc).
