// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"encoding/json"
	"fmt"

	"sigs.k8s.io/yaml"
)

// Decode parses an Extension's providerConfig. Unknown fields are rejected so a typo in a
// Shoot fails the reconcile instead of being silently ignored.
func Decode(raw []byte) (*SOCConfig, error) {
	cfg := &SOCConfig{}
	if err := yaml.UnmarshalStrict(raw, cfg); err != nil {
		return nil, fmt.Errorf("providerConfig is not a valid %s: %w", Kind, err)
	}
	if cfg.APIVersion != GroupVersion || cfg.Kind != Kind {
		return nil, fmt.Errorf("providerConfig must be %s %s, got %q %q", GroupVersion, Kind, cfg.APIVersion, cfg.Kind)
	}
	switch cfg.Mode {
	case "", ModeSelfContained, ModeFull:
	default:
		return nil, fmt.Errorf("providerConfig mode %q: want %s or %s", cfg.Mode, ModeSelfContained, ModeFull)
	}
	if cfg.Mode == ModeFull && (cfg.ClickHouse == nil || cfg.ClickHouse.CentralHost == nil) {
		return nil, fmt.Errorf("mode %s needs clickhouse.centralHost", ModeFull)
	}
	if cfg.Detection != nil && cfg.Detection.Mode != nil {
		switch *cfg.Detection.Mode {
		case "default", "alert", "enforce":
		default:
			return nil, fmt.Errorf("detection.mode %q: want default, alert or enforce", *cfg.Detection.Mode)
		}
	}
	return cfg, nil
}

// Values renders the overrides as chart values; nil fields produce no key.
func (c *SOCConfig) Values() map[string]any {
	out := map[string]any{}
	set := func(section, key string, v any) {
		m, ok := out[section].(map[string]any)
		if !ok {
			m = map[string]any{}
			out[section] = m
		}
		m[key] = v
	}
	switch c.Mode {
	case ModeFull:
		set("clickhouse", "mode", "central")
	case ModeSelfContained:
		set("clickhouse", "mode", "local")
	}
	if p := c.Pixie; p != nil {
		for k, v := range map[string]*string{"cloudAddr": p.CloudAddr, "vizierVersion": p.VizierVersion, "dataAccess": p.DataAccess, "pemMemoryLimit": p.PemMemoryLimit, "pemMemoryRequest": p.PemMemoryRequest} {
			if v != nil {
				set("pixie", k, *v)
			}
		}
	}
	if ch := c.ClickHouse; ch != nil {
		if ch.RetentionHours != nil {
			set("clickhouse", "retentionHours", *ch.RetentionHours)
		}
		if ch.Storage != nil {
			set("clickhouse", "storage", *ch.Storage)
		}
		if ch.CentralHost != nil {
			set("clickhouse", "central", map[string]any{"host": *ch.CentralHost})
		}
	}
	if d := c.Detection; d != nil {
		if d.Mode != nil {
			set("kubescape", "mode", *d.Mode)
		}
		if d.Rules != nil {
			set("kubescape", "rules", d.Rules)
		}
		if d.ExcludeNamespaces != nil {
			set("kubescape", "excludeNamespaces", d.ExcludeNamespaces)
		}
	}
	if co := c.Components; co != nil {
		for k, v := range map[string]*bool{"kubescape": co.Kubescape, "vector": co.Vector, "adaptiveExport": co.AdaptiveExport, "dx": co.Dx} {
			if v != nil {
				set(k, "enabled", *v)
			}
		}
	}
	// Profiles are not copied here: BuildSOCValues has to fetch them, and a half-populated
	// profiles section in the values would be indistinguishable from a fetch that failed.
	if len(c.Images) > 0 {
		imgs := map[string]any{}
		for k, v := range c.Images {
			imgs[k] = v
		}
		out["images"] = imgs
	}
	return out
}

// ToJSON exists for tests and logging.
func (c *SOCConfig) ToJSON() string {
	b, _ := json.Marshal(c)
	return string(b)
}
