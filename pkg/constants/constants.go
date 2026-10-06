// SPDX-License-Identifier: Apache-2.0

package constants

const (
	// ExtensionType is the name of the extension type: what a Shoot's spec.extensions[].type
	// must say, and what the lifecycle controller's predicate matches. Everything derived
	// below — service name, ManagedResource names, finalizer, healthcheck registration —
	// follows from it.
	ExtensionType = "shoot-rtsecurity-service"

	ServiceName                  = ExtensionType
	ExtensionServiceName         = "extension-" + ServiceName
	GardenerExtensionServiceName = "gardener-" + ExtensionServiceName

	// ManagedResourceNameShoot holds the SOC stack rendered into the shoot.
	ManagedResourceNameShoot = ExtensionServiceName + "-shoot"

	// ManagedResourceNameSeed is only ever deleted: see actuator.createSeedResources.
	ManagedResourceNameSeed = ExtensionServiceName + "-seed"

	// SOCChartname is the chart this extension renders: the SOC runtime-security stack
	// (ClickHouse, kubescape node-agent, vector, pixie vizier/PEM, adaptive-export,
	// dx-daemon).
	SOCChartname = "soc"
)
