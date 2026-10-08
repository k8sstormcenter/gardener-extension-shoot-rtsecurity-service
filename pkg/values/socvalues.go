// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0
package values

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	socv1alpha1 "github.com/k8sstormcenter/gardener-extension-shoot-rtsecurity-service/pkg/apis/soc/v1alpha1"
	"github.com/k8sstormcenter/gardener-extension-shoot-rtsecurity-service/pkg/profiles"
	"github.com/k8sstormcenter/gardener-extension-shoot-rtsecurity-service/pkg/utils"
)

// ConfigBuilder turns a reconcile context into chart values.
type ConfigBuilder struct {
	client   client.Client
	profiles *profiles.Fetcher
}

func NewConfigBuilder(client client.Client) *ConfigBuilder {
	return &ConfigBuilder{client: client, profiles: profiles.NewFetcher()}
}

// BuildSOCValues returns only the values that have to be computed per shoot. Everything
// else comes from charts/internal/soc/values.yaml: helm merges the chart's own defaults
// with what is supplied here, so this deliberately does not restate them. A value that
// appears in both places is a value with two owners.
func (c *ConfigBuilder) BuildSOCValues(ctx context.Context, reconcileCtx *utils.ReconcileContext) (map[string]any, error) {
	clusterName := reconcileCtx.ClusterName
	if clusterName == "" && reconcileCtx.Shoot != nil {
		clusterName = reconcileCtx.Shoot.Name
	}

	// Deliberately an error rather than a fallback: the pixie cloud registers a new cluster
	// for every unrecognised name, so a nameless install leaves an orphan registration
	// behind. Failing the reconcile is cheaper than cleaning that up.
	if clusterName == "" {
		return nil, fmt.Errorf("cannot derive a pixie cluster name: both ReconcileContext.ClusterName and Shoot are empty")
	}

	refs := defaultCredentialRefs
	if reconcileCtx.SOCConfig != nil && reconcileCtx.SOCConfig.Credentials != nil {
		refs = refs.override(reconcileCtx.SOCConfig.Credentials)
	}
	creds, err := c.credentialValues(ctx, reconcileCtx, refs)
	if err != nil {
		return nil, err
	}

	values := map[string]any{}
	if reconcileCtx.SOCConfig != nil {
		values = reconcileCtx.SOCConfig.Values()
	}
	if err := c.addProfiles(ctx, reconcileCtx, values); err != nil {
		return nil, err
	}
	pixie, _ := values["pixie"].(map[string]any)
	if pixie == nil {
		pixie = map[string]any{}
	}
	pixie["clusterName"] = clusterName
	values["pixie"] = pixie
	values["credentials"] = creds
	return values, nil
}

type credentialRefs struct {
	deployKey, apiKey, pullEntlein, pullTanzeee, arcToken string
}

var defaultCredentialRefs = credentialRefs{RefPixieDeployKey, RefPixieAPIKey, RefPullEntlein, RefPullTanzeee, RefArcGithubToken}

func (r credentialRefs) override(c *socv1alpha1.Credentials) credentialRefs {
	pick := func(cur string, v *string) string {
		if v != nil && *v != "" {
			return *v
		}
		return cur
	}
	return credentialRefs{pick(r.deployKey, c.PixieDeployKey), pick(r.apiKey, c.PixieAPIKey), pick(r.pullEntlein, c.PullEntlein), pick(r.pullTanzeee, c.PullTanzeee), pick(r.arcToken, c.ArcGithubToken)}
}

// addProfiles fetches the signed-off profiles and rules and puts them in the values as
// whole documents. They are rendered verbatim: a profile is evidence that was reviewed in
// a pull request, and templating it here would mean the thing running in the shoot is not
// the thing that was signed off.
func (c *ConfigBuilder) addProfiles(ctx context.Context, reconcileCtx *utils.ReconcileContext, values map[string]any) error {
	if reconcileCtx.SOCConfig == nil || reconcileCtx.SOCConfig.Profiles == nil {
		return nil
	}
	p := reconcileCtx.SOCConfig.Profiles

	var token string
	if p.TokenRef != nil && *p.TokenRef != "" {
		s, err := c.referencedSecret(ctx, reconcileCtx, *p.TokenRef)
		if err != nil {
			return err
		}
		if s == nil {
			return fmt.Errorf("profiles.tokenRef %q is not declared in the Shoot's spec.resources", *p.TokenRef)
		}
		for _, v := range s.Data {
			token = string(v)
			break
		}
	}

	docs, err := c.profiles.Fetch(ctx, profiles.Source{Repo: p.Repo, Ref: p.Ref, Path: p.Path, Token: token})
	if err != nil {
		return err
	}
	out := make([]any, 0, len(docs))
	for _, d := range docs {
		out = append(out, map[string]any{"name": d.Name, "content": d.Content})
	}
	values["profiles"] = map[string]any{"enabled": true, "documents": out}
	return nil
}

// Referenced-resource names a Shoot must use in spec.resources[] to hand this extension its
// credentials. Fixed by convention until the SOC providerConfig type exists; gardener copies
// each referenced Secret into the shoot namespace as "ref-<secret name>", which is where
// the actuator reads them. Every ref is optional: a missing one renders the stack without
// that credential rather than failing, because a cluster using a public mirror is valid.
const (
	RefPixieDeployKey = "soc-pixie-deploy-key"
	RefPixieAPIKey    = "soc-pixie-api-key"
	RefPullEntlein    = "soc-pull-entlein"
	RefPullTanzeee    = "soc-pull-tanzeee"
	RefArcGithubToken = "soc-arc-github-token"
)

// referencedSecret resolves one NamedResourceReference to the Secret gardener mirrored into
// the shoot namespace. nil, nil when the Shoot did not declare the ref.
func (c *ConfigBuilder) referencedSecret(ctx context.Context, reconcileCtx *utils.ReconcileContext, refName string) (*corev1.Secret, error) {
	for _, ref := range reconcileCtx.ResourceSection {
		if ref.Name != refName || ref.ResourceRef.Kind != "Secret" || ref.ResourceRef.APIVersion != "v1" {
			continue
		}
		s := &corev1.Secret{}
		if err := c.client.Get(ctx, client.ObjectKey{Namespace: reconcileCtx.Namespace, Name: "ref-" + ref.ResourceRef.Name}, s); err != nil {
			return nil, fmt.Errorf("resource %q is declared but ref-%s is not readable in %s: %w", refName, ref.ResourceRef.Name, reconcileCtx.Namespace, err)
		}
		return s, nil
	}
	return nil, nil
}

// credentialValues fills values.credentials from the referenced Secrets. Pull secrets are
// passed as the raw .dockerconfigjson so the chart writes kubernetes.io/dockerconfigjson
// Secrets unchanged; the deploy key is the single data value whatever its key is called.
func (c *ConfigBuilder) credentialValues(ctx context.Context, reconcileCtx *utils.ReconcileContext, refs credentialRefs) (map[string]any, error) {
	out := map[string]any{}
	for ref, key := range map[string]string{refs.pullEntlein: "ducklingPullSecret", refs.pullTanzeee: "tanzeeePullSecret"} {
		s, err := c.referencedSecret(ctx, reconcileCtx, ref)
		if err != nil {
			return nil, err
		}
		if s == nil {
			continue
		}
		cfg, ok := s.Data[corev1.DockerConfigJsonKey]
		if !ok {
			return nil, fmt.Errorf("resource %q must be a kubernetes.io/dockerconfigjson secret (missing %s)", ref, corev1.DockerConfigJsonKey)
		}
		out[key] = string(cfg)
	}
	for ref, key := range map[string]string{refs.deployKey: "pixieDeployKey", refs.apiKey: "pixieApiKey", refs.arcToken: "arcGithubToken"} {
		s, err := c.referencedSecret(ctx, reconcileCtx, ref)
		if err != nil {
			return nil, err
		}
		if s == nil {
			continue
		}
		for _, v := range s.Data {
			out[key] = string(v)
			break
		}
	}
	return out, nil
}
