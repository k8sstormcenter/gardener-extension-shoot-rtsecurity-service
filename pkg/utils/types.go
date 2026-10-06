// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0
package utils

import (
	gardenerv1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	socv1alpha1 "github.com/k8sstormcenter/gardener-extension-shoot-rtsecurity-service/pkg/apis/soc/v1alpha1"
)

type ReconcileContext struct {
	Namespace               string
	ExtensionClass          *extensionsv1alpha1.ExtensionClass
	TargetClusterK8sVersion string
	ResourceSection         []gardenerv1beta1.NamedResourceReference
	ClusterIdentity         *string
	SOCConfig               *socv1alpha1.SOCConfig
	ShootTechnicalId        string
	SeedIngressDomain       string
	ClusterName             string
	IsSeedDeployment        bool
	IsShootDeployment       bool
	IsGardenDeployment      bool
	Shoot                   *gardenerv1beta1.Shoot
	Seed                    *gardenerv1beta1.Seed
}
