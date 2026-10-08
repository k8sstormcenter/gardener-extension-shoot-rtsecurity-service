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
	Profiles    *Profiles    `json:"profiles,omitempty"`
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
	// ExcludeNamespaces replaces the namespaces the sensor ignores, which is also what the
	// alert binding leaves out: rules bound where the sensor does not look cannot fire.
	ExcludeNamespaces []string `json:"excludeNamespaces,omitempty"`
}

type Components struct {
	Kubescape      *bool `json:"kubescape,omitempty"`
	AdaptiveExport *bool `json:"adaptiveExport,omitempty"`
	Dx             *bool `json:"dx,omitempty"`
}

// Profiles points at the git location holding the signed-off profiles and rules for this
// shoot. The actuator fetches them at reconcile and renders them into the same
// ManagedResource as the rest of the stack, so the shoot has exactly one owner.
//
// Ref may be a branch, tag, commit or a pull-request head (refs/pull/<n>/head). Tracking a
// PR head is the point: the shoot runs the proposed profiles while the change is still
// under review, and merging the PR is what makes them permanent rather than what first
// applies them.
type Profiles struct {
	// Repo is "<owner>/<name>" on GitHub.
	Repo string `json:"repo,omitempty"`
	// Ref defaults to the repository's default branch.
	Ref string `json:"ref,omitempty"`
	// Path is the directory in the repo to read, without a leading slash. Only *.yaml and
	// *.yml directly inside it are read; subdirectories are ignored so that a per-shoot
	// directory cannot silently pull in another shoot's profiles.
	Path string `json:"path,omitempty"`
	// TokenRef names the Shoot spec.resources[] entry holding a token for a private repo.
	// A public repo needs none.
	TokenRef *string `json:"tokenRef,omitempty"`
}

// Credentials names the Shoot's spec.resources[] entries to read instead of the defaults
// soc-pixie-deploy-key, soc-pixie-api-key, soc-pull-entlein, soc-pull-tanzeee,
// soc-arc-github-token.
type Credentials struct {
	PixieDeployKey *string `json:"pixieDeployKey,omitempty"`
	PixieAPIKey    *string `json:"pixieApiKey,omitempty"`
	PullEntlein    *string `json:"pullEntlein,omitempty"`
	PullTanzeee    *string `json:"pullTanzeee,omitempty"`
	ArcGithubToken *string `json:"arcGithubToken,omitempty"`
}
