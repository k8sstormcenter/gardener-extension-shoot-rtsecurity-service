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
	Target      *Target      `json:"target,omitempty"`
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

	// LearnOnlyNamespaces names namespaces the sensor watches and the alert binding leaves
	// out: learned, never judged. It is how a cluster profiles its own system workloads
	// without alerting on them.
	//
	// It sits here rather than under a kubescape section of its own because it only has
	// meaning against ExcludeNamespaces above, and the chart subtracts these names from
	// that list. A caller therefore does not have to know what the chart's default
	// exclusion contains in order to take one name out of it.
	LearnOnlyNamespaces []string `json:"learnOnlyNamespaces,omitempty"`
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
// Target is the workload the signed-off profiles govern and the attack suite is scored
// against. It belongs to the shoot rather than to the chart: a scale set's name is a
// property of one cluster, and a chart default naming one cluster's workload is how a
// suite ends up aimed at the cluster it was authored against.
type Target struct {
	// Enabled brings the workload's namespace under the sensor. Until a signed-off profile
	// exists, leaving it off is deliberate: an ungoverned scale set bursting to hundreds of
	// pods emits an alert per distinct exec.
	Enabled *bool `json:"enabled,omitempty"`
	// Namespace the workload runs in.
	Namespace *string `json:"namespace,omitempty"`
	// ProfileName is the kubescape.io/user-defined-profile label value its pods carry, and
	// therefore the name the agent resolves per container as "<profileName>-<container>".
	// It is NOT the workload's name: the label and the pod-name prefix are different
	// strings, and one field serving both breaks whichever it is not.
	ProfileName *string `json:"profileName,omitempty"`
	// ScaleSetName is the workload's own name, which is the prefix its pods are named with.
	// dx scores pods matching "<namespace>/<scaleSetName>-runner-%".
	ScaleSetName *string `json:"scaleSetName,omitempty"`

	// PodPrefix is the pod-name prefix dx scores on, stated outright instead of being derived
	// from ScaleSetName above. It belongs here because it overrides exactly that field: the
	// chart derives the prefix from the declared scale set, and a cluster running the workload
	// under several scale sets has no single name to derive from.
	//
	// dx matches it as SQL LIKE, so "%" spans the part that varies between them. Bare, never
	// namespace-qualified: the namespace comes from Namespace above and the chart joins the
	// two, so a qualified value would be composed twice and match nothing. The render refuses
	// one that contains "/".
	PodPrefix *string `json:"podPrefix,omitempty"`
	// Containers and InitContainers are every container its pods run. A container no
	// delivered profile names is unresolved, which the sensor treats as deny-all, so the
	// render refuses a delivery that does not cover each one.
	Containers     []string `json:"containers,omitempty"`
	InitContainers []string `json:"initContainers,omitempty"`
}

type Profiles struct {
	// Repo is "<owner>/<name>" on GitHub.
	Repo string `json:"repo,omitempty"`
	// Ref defaults to the repository's default branch.
	Ref string `json:"ref,omitempty"`
	// Path is the directory in the repo to read, without a leading slash. Only *.yaml and
	// *.yml directly inside it are read; subdirectories are ignored so that a per-shoot
	// directory cannot silently pull in another shoot's profiles.
	Path string `json:"path,omitempty"`
	// SuitePath is one file in the same repo, read at the same Ref, used as dx's attack
	// suite. One ref for both means the suite and the profiles it scores against cannot
	// drift. Empty leaves the chart's bundled proof-suite.yaml in place.
	SuitePath string `json:"suitePath,omitempty"`
	// RulesPath is one file in the same repo, read at the same Ref, holding the signed-off
	// kubescape Rules object. It REPLACES the chart's default-rules rather than joining it:
	// the names in a rule set are not the rule set, so the tuning a review approved (opcode
	// filters, dedup windows, per-rule supportPolicy) only reaches the sensor as the whole
	// document. Empty leaves the chart's own rules in place.
	RulesPath string `json:"rulesPath,omitempty"`
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
