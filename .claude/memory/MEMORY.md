# Memory Index

## Local Development
- [Local Dev Environment](local-dev-environment.md) — kubeconfig paths, deploy commands, shoot management. Note the chart is baked into the extension image: any chart change needs a full `make extension-up`.
- [Local Dev Troubleshooting](local-dev-troubleshooting.md) — secretRef-not-in-spec.resources, store reachability from the shoot, disk pressure, stuck machines

## Extension Development
- Component pins live in `charts/internal/soc/values.yaml` (images) and the kubescape render in `templates/kubescape-operator.yaml`; both must move together.
- [SOC Chart Port](soc-chart-port.md) — what the skaffolds did imperatively and where each piece goes
