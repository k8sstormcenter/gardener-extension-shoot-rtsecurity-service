// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"os"

	healthcheckconfig "github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	apisconfig "github.com/k8sstormcenter/gardener-extension-shoot-rtsecurity-service/pkg/apis/config"
	controllerconfig "github.com/k8sstormcenter/gardener-extension-shoot-rtsecurity-service/pkg/controller/config"
)

var (
	scheme  *runtime.Scheme
	decoder runtime.Decoder
)

func init() {
	scheme = runtime.NewScheme()
	utilruntime.Must(apisconfig.AddToScheme(scheme))

	// Deserializer, not UniversalDecoder: the config has a single version, so there is no
	// internal version to convert to and a converting decoder would fail to find one.
	decoder = serializer.NewCodecFactory(scheme).UniversalDeserializer()
}

// ExtensionOptions are the command line options of the extension controller.
type ExtensionOptions struct {
	ConfigLocation string
	config         *ExtensionConfig
}

type ExtensionConfig struct {
	config apisconfig.Configuration
}

// Complete implements Completer.Complete.
func (o *ExtensionOptions) Complete() error {
	if o.ConfigLocation == "" {
		return errors.New("config location is not set")
	}
	data, err := os.ReadFile(o.ConfigLocation)
	if err != nil {
		return err
	}

	config := apisconfig.Configuration{}
	if _, _, err := decoder.Decode(data, nil, &config); err != nil {
		return err
	}

	o.config = &ExtensionConfig{config: config}
	return nil
}

// Completed returns the completed Config. Only call this if `Complete` was successful.
func (o *ExtensionOptions) Completed() *ExtensionConfig {
	return o.config
}

// AddFlags implements Flagger.AddFlags.
func (o *ExtensionOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.ConfigLocation, "config-file", "", "path to the controller manager configuration file")
}

// Apply sets the values of this Config in the given controller configuration.
func (c *ExtensionConfig) Apply(config *controllerconfig.Config) {
	config.Configuration = c.config
}

// Configuration returns the parsed configuration.
func (c *ExtensionConfig) Configuration() *apisconfig.Configuration {
	return &c.config
}

// ApplyHealthCheckConfig applies the HealthCheckConfig to the config.
func (c *ExtensionConfig) ApplyHealthCheckConfig(config *healthcheckconfig.HealthCheckConfig) {
	if c.config.HealthCheckConfig != nil {
		*config = *c.config.HealthCheckConfig
	}
}
