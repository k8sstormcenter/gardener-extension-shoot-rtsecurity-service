// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0
package values

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	socv1alpha1 "github.com/gardener/gardener-extension-shoot-falco-service/pkg/apis/soc/v1alpha1"
	"github.com/gardener/gardener-extension-shoot-falco-service/pkg/utils"
)

// BuildSOCValues returns only the values that have to be computed per shoot. Everything
// else comes from charts/internal/soc/values.yaml: helm merges the chart's own defaults
// with what is supplied here, so this deliberately does not restate them. A value that
// appears in both places is a value with two owners.
func (c *ConfigBuilder) BuildSOCValues(ctx context.Context, reconcileCtx *utils.ReconcileContext) (map[string]any, error) {
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
	deployKey, apiKey, pullEntlein, pullTanzeee string
}

var defaultCredentialRefs = credentialRefs{RefPixieDeployKey, RefPixieAPIKey, RefPullEntlein, RefPullTanzeee}

func (r credentialRefs) override(c *socv1alpha1.Credentials) credentialRefs {
	pick := func(cur string, v *string) string {
		if v != nil && *v != "" {
			return *v
		}
		return cur
	}
	return credentialRefs{pick(r.deployKey, c.PixieDeployKey), pick(r.apiKey, c.PixieAPIKey), pick(r.pullEntlein, c.PullEntlein), pick(r.pullTanzeee, c.PullTanzeee)}
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
	for ref, key := range map[string]string{refs.deployKey: "pixieDeployKey", refs.apiKey: "pixieApiKey"} {
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
