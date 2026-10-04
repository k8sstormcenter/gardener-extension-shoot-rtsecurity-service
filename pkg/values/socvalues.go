// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0
package values

import (
	"fmt"

	"github.com/gardener/gardener-extension-shoot-falco-service/pkg/utils"
)

// BuildSOCValues returns only the values that have to be computed per shoot. Everything
// else comes from charts/internal/soc/values.yaml: helm merges the chart's own defaults
// with what is supplied here, so this deliberately does not restate them. A value that
// appears in both places is a value with two owners.
func (c *ConfigBuilder) BuildSOCValues(reconcileCtx *utils.ReconcileContext) (map[string]any, error) {
	clusterName := reconcileCtx.ClusterName
	if clusterName == "" && reconcileCtx.Shoot != nil {
		clusterName = reconcileCtx.Shoot.Name
	}

	// Deliberately an error rather than a fallback. The pixie cloud namespace is flat and
	// registers a new cluster for every unrecognised name: the fleet already carries six
	// default_<hash> registrations, five of them dead, because one install was allowed to
	// proceed without a name. Failing the reconcile is cheaper than another orphan.
	if clusterName == "" {
		return nil, fmt.Errorf("cannot derive a pixie cluster name: both ReconcileContext.ClusterName and Shoot are empty")
	}

	return map[string]any{
		"pixie": map[string]any{
			"clusterName": clusterName,
		},
	}, nil
}
