// SPDX-License-Identifier: Apache-2.0

// Package config is the extension controller's own configuration, read from the file named
// by --config-file. One version only, so there is no internal/external split and nothing
// to convert; the type is hand-written for the same reason.
package config

import (
	healthcheckconfigv1alpha1 "github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const GroupName = "rtsecurity.extensions.config.gardener.cloud"

var (
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}
	SchemeBuilder      = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme        = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion, &Configuration{})
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

// Configuration is the extension controller configuration.
type Configuration struct {
	metav1.TypeMeta `json:",inline"`

	// HealthCheckConfig is the config for the health check controller.
	HealthCheckConfig *healthcheckconfigv1alpha1.HealthCheckConfig `json:"healthCheckConfig,omitempty"`
}

func (c *Configuration) DeepCopy() *Configuration {
	if c == nil {
		return nil
	}
	out := *c
	if c.HealthCheckConfig != nil {
		out.HealthCheckConfig = c.HealthCheckConfig.DeepCopy()
	}
	return &out
}

func (c *Configuration) DeepCopyObject() runtime.Object {
	if c == nil {
		return nil
	}
	return c.DeepCopy()
}
