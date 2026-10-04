// SPDX-License-Identifier: Apache-2.0

// Package v1alpha1 is the providerConfig of the shoot-rtsecurity-service extension: the
// settings a shoot owner may change about the SOC stack. Every field is optional and nil
// means "the chart default" (charts/internal/soc/values.yaml), so the chart stays the
// single owner of defaults and this type only carries overrides.
package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	GroupVersion = "rtsecurity.extensions.gardener.cloud/v1alpha1"
	Kind         = "SOCConfig"

	// ModeSelfContained runs a ClickHouse inside the shoot; ModeFull ships to the central
	// forensic ClickHouse and runs none locally.
	ModeSelfContained = "selfcontained"
	ModeFull          = "full"
)

type SOCConfig struct {
	metav1.TypeMeta `json:",inline"`

	Mode string `json:"mode,omitempty"`

	Pixie       *Pixie       `json:"pixie,omitempty"`
	ClickHouse  *ClickHouse  `json:"clickhouse,omitempty"`
	Detection   *Detection   `json:"detection,omitempty"`
	Components  *Components  `json:"components,omitempty"`
	Credentials *Credentials `json:"credentials,omitempty"`
	// Image overrides keyed like values.images (pixieOperator, nodeAgent, ...).
	Images map[string]string `json:"images,omitempty"`
}

type Pixie struct {
	CloudAddr        *string `json:"cloudAddr,omitempty"`
	VizierVersion    *string `json:"vizierVersion,omitempty"`
	DataAccess       *string `json:"dataAccess,omitempty"`
	PemMemoryLimit   *string `json:"pemMemoryLimit,omitempty"`
	PemMemoryRequest *string `json:"pemMemoryRequest,omitempty"`
}

type ClickHouse struct {
	RetentionHours *int    `json:"retentionHours,omitempty"`
	Storage        *string `json:"storage,omitempty"`
	// Host of the central ClickHouse; used in ModeFull only.
	CentralHost *string `json:"centralHost,omitempty"`
}

type Detection struct {
	// Mode is the node-agent posture: default | alert | enforce.
	Mode *string `json:"mode,omitempty"`
	// Rules replaces the bound rule set (names from soc's default-rules).
	Rules []string `json:"rules,omitempty"`
	// ExcludeNamespaces replaces the sensor's ignore list.
	ExcludeNamespaces []string `json:"excludeNamespaces,omitempty"`
	// BindingExcludeNamespaces replaces the alert binding's NotIn list.
	BindingExcludeNamespaces []string `json:"bindingExcludeNamespaces,omitempty"`
}

type Components struct {
	Kubescape      *bool `json:"kubescape,omitempty"`
	Vector         *bool `json:"vector,omitempty"`
	AdaptiveExport *bool `json:"adaptiveExport,omitempty"`
	Dx             *bool `json:"dx,omitempty"`
}

// Credentials names the Shoot's spec.resources[] entries to read instead of the defaults
// soc-pixie-deploy-key, soc-pixie-api-key, soc-pull-entlein, soc-pull-tanzeee.
type Credentials struct {
	PixieDeployKey *string `json:"pixieDeployKey,omitempty"`
	PixieAPIKey    *string `json:"pixieApiKey,omitempty"`
	PullEntlein    *string `json:"pullEntlein,omitempty"`
	PullTanzeee    *string `json:"pullTanzeee,omitempty"`
}
