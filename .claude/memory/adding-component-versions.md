---
name: adding-component-versions
description: "How to add a SOC component version (kubescape chart, node-agent, vector, AE, dx, pixie) to the extension — which files to edit and what must agree"
metadata: 
  node_type: memory
  type: reference
  originSessionId: 1dabdd3f-b2ee-4599-af9d-c4de6be5ce1e
---

## Adding a component version

Our components are not falco's. The mechanism below is unchanged — four places that must
agree — but the files and the component list are ours: the kubescape operator chart,
node-agent, vector, adaptive-export, dx-daemon and the pixie vizier/operator pins.

Pins come from the chart release you intend, NOT from whatever a cluster happens to run.
edge4 and k3s-1 have sat on different PEM/AE lines, so "what edge4 runs" is not a target.

### Files to edit (all relative to repo root)

1. **`imagevector/images.yaml`** — Add image entry with tag, version, CVE labels
2. **`falco/falcoversions.yaml`** — Add version with classification and rulesVersion
3. **`falco/falco-profile.yaml`** — Add to both `spec.images.falco[]` and `spec.versions.falco[]`
4. **`falco/rules/<version>/`** — Download rules files

### Classification values

- `deprecated` — version will be removed, requires `expirationDate`
- `supported` — production-ready, auto-update target
- `preview` — available but not auto-updated to

### Fetching rules

Rules are tagged in `falcosecurity/rules` repo. Find the tag from the Falco release notes (e.g. Falco 0.44.0 uses `falco-rules-5.1.0`):

```bash
mkdir -p falco/rules/<version>
cd falco/rules/<version>
curl -sL "https://raw.githubusercontent.com/falcosecurity/rules/falco-rules-<rules-tag>/rules/falco_rules.yaml" -o falco_rules.yaml
curl -sL "https://raw.githubusercontent.com/falcosecurity/rules/falco-rules-<rules-tag>/rules/falco-incubating_rules.yaml" -o falco-incubating_rules.yaml
curl -sL "https://raw.githubusercontent.com/falcosecurity/rules/falco-rules-<rules-tag>/rules/falco-sandbox_rules.yaml" -o falco-sandbox_rules.yaml
```

The `rulesVersion` in `falcoversions.yaml` maps to the directory name under `falco/rules/`.

### Adding a new falcosidekick version

Same pattern but simpler (no rules):
1. **`imagevector/images.yaml`** — Add falcosidekick image entry
2. **`falco/falcosidekickversions.yaml`** — Add version + classification
3. **`falco/falco-profile.yaml`** — Add to `spec.images.falcosidekick[]` and `spec.versions.falcosidekick[]`

### Auto-update behavior

- Falcosidekick marked `supported` will be auto-selected on reconcile (latest supported wins)
- Falco version must be explicitly set by the user in FalcoServiceConfig unless `autoUpdate: true`
- `preview` versions are never auto-selected

### Regenerating falco-profile.yaml (optional)

```bash
make generate-profile
```

This reads `imagevector/images.yaml`, `falco/falcoversions.yaml`, and `falco/falcosidekickversions.yaml` to produce `falco/falco-profile.yaml`. Can also be edited manually.

### Verifying

```bash
go build ./...
make validate-imagevector
```
