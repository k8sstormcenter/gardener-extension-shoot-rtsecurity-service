# Memory Index

## Local Development
- [Local Dev Environment](local-dev-environment.md) — kubeconfig paths, deploy commands, shoot management. Note the chart is baked into the extension image: any chart change needs a full `make extension-up`.
- [Local Dev Troubleshooting](local-dev-troubleshooting.md) — secretRef-not-in-spec.resources, store reachability from the shoot, disk pressure, stuck machines
- [E2E Testing the SOC stack](e2e-testing-clickhouse.md) — shoot manifest, ClickHouse in-shoot, event verification

## Extension Development
- [Adding Component Versions](adding-component-versions.md) — kubescape chart, node-agent, vector, AE, dx, pixie pins: four files that must agree
- [SOC Chart Port](soc-chart-port.md) — what the skaffolds did imperatively and where each piece goes
