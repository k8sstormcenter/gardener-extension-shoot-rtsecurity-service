// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/extension"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/api/extensions/v1alpha1/helper"
	gardenerv1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	managedresources "github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/k8sstormcenter/gardener-extension-shoot-rtsecurity-service/charts"
	"github.com/k8sstormcenter/gardener-extension-shoot-rtsecurity-service/pkg/apis/config"
	socv1alpha1 "github.com/k8sstormcenter/gardener-extension-shoot-rtsecurity-service/pkg/apis/soc/v1alpha1"
	"github.com/k8sstormcenter/gardener-extension-shoot-rtsecurity-service/pkg/constants"
	"github.com/k8sstormcenter/gardener-extension-shoot-rtsecurity-service/pkg/utils"
	"github.com/k8sstormcenter/gardener-extension-shoot-rtsecurity-service/pkg/values"
)

// NewActuator returns an actuator responsible for Extension resources.
func NewActuator(mgr manager.Manager, serviceConfig config.Configuration) (extension.Actuator, error) {
	// A file path, not an in-cluster config: the garden is reached with the pod's own
	// service-account token mounted as a kubeconfig (see the chart's tokenFile kubeconfig).
	gardenRESTConfig, err := kubernetes.RESTConfigFromKubeconfigFile(os.Getenv("GARDEN_KUBECONFIG"), kubernetes.AuthTokenFile)
	if err != nil {
		return nil, err
	}
	dynamicGardenCluster, err := dynamic.NewForConfig(gardenRESTConfig)
	if err != nil {
		return nil, fmt.Errorf("failed creating dynamic garden cluster object: %w", err)
	}

	localClusterK8sVersion, err := getLocalClusterK8sVersion(mgr.GetConfig())
	if err != nil {
		return nil, err
	}
	seed, err := utils.GetSeed(context.TODO(), dynamicGardenCluster, os.Getenv("SEED_NAME"))
	if err != nil {
		return nil, fmt.Errorf("cannot get seed: %v", err)
	}

	return &actuator{
		client:                 mgr.GetClient(),
		config:                 mgr.GetConfig(),
		serviceConfig:          serviceConfig,
		configBuilder:          values.NewConfigBuilder(mgr.GetClient()),
		gardenClient:           dynamicGardenCluster,
		localClusterK8sVersion: localClusterK8sVersion,
		seed:                   seed,
	}, nil
}

type actuator struct {
	client                 client.Client
	config                 *rest.Config
	serviceConfig          config.Configuration
	configBuilder          *values.ConfigBuilder
	gardenClient           *dynamic.DynamicClient
	localClusterK8sVersion string
	seed                   *gardenerv1beta1.Seed
}

// Reconcile the Extension resource.
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	var (
		reconcileCtx *utils.ReconcileContext
		err          error
		namespace    = ex.GetNamespace()
	)

	switch extensionsv1alpha1helper.GetExtensionClassOrDefault(ex.Spec.Class) {
	case extensionsv1alpha1.ExtensionClassShoot:
		shootCluster, err := controller.GetCluster(ctx, a.client, namespace)
		if err != nil {
			return fmt.Errorf("failed to get cluster config for shoot: %w", err)
		}
		if controller.IsHibernated(shootCluster) {
			return nil
		}
		reconcileCtx = &utils.ReconcileContext{
			TargetClusterK8sVersion: shootCluster.Shoot.Spec.Kubernetes.Version,
			ResourceSection:         shootCluster.Shoot.Spec.Resources,
			ClusterIdentity:         shootCluster.Shoot.Status.ClusterIdentity,
			ShootTechnicalId:        shootCluster.Shoot.Status.TechnicalID,
			ClusterName:             shootCluster.Shoot.Name,
			Shoot:                   shootCluster.Shoot,
			Seed:                    shootCluster.Seed,
		}
		if shootCluster.Seed.Spec.Ingress != nil {
			reconcileCtx.SeedIngressDomain = shootCluster.Seed.Spec.Ingress.Domain
		}
	case extensionsv1alpha1.ExtensionClassSeed:
		// The stack is deployed onto the seed itself, whose version is known locally.
		reconcileCtx = &utils.ReconcileContext{
			TargetClusterK8sVersion: a.localClusterK8sVersion,
			ResourceSection:         a.seed.Spec.Resources,
			ClusterIdentity:         a.seed.Status.ClusterIdentity,
			ClusterName:             a.seed.Name,
		}
	case extensionsv1alpha1.ExtensionClassGarden:
		reconcileCtx = &utils.ReconcileContext{
			TargetClusterK8sVersion: a.localClusterK8sVersion,
			ClusterName:             "garden",
		}
	default:
		return fmt.Errorf("unsupported extension class %q", *ex.Spec.Class)
	}

	if ex.Spec.ProviderConfig != nil {
		if reconcileCtx.SOCConfig, err = socv1alpha1.Decode(ex.Spec.ProviderConfig.Raw); err != nil {
			return err
		}
	}
	reconcileCtx.Namespace = namespace
	reconcileCtx.IsSeedDeployment = isSeedDeployment(ex)
	reconcileCtx.IsShootDeployment = isShootDeployment(ex)
	reconcileCtx.IsGardenDeployment = isGardenDeployment(ex)

	if err := a.createShootResources(ctx, log, reconcileCtx); err != nil {
		return err
	}
	return a.createSeedResources(ctx, log, namespace)
}

func (a *actuator) createShootResources(ctx context.Context, log logr.Logger, reconcileCtx *utils.ReconcileContext) error {
	log.Info("creating SOC resources for shoot " + reconcileCtx.Namespace)
	renderer, err := util.NewChartRendererForShoot(reconcileCtx.TargetClusterK8sVersion)
	if err != nil {
		return fmt.Errorf("could not create chart renderer for rendering manged resource chart for shoot: %w", err)
	}
	chartValues, err := a.configBuilder.BuildSOCValues(ctx, reconcileCtx)
	if err != nil {
		return fmt.Errorf("could not generate SOC configuration: %w", err)
	}
	release, err := renderer.RenderEmbeddedFS(charts.InternalChart, filepath.Join(charts.InternalChartsPath, constants.SOCChartname), constants.SOCChartname, metav1.NamespaceSystem, chartValues)
	if err != nil {
		return fmt.Errorf("could not render chart for shoot: %w", err)
	}

	data := map[string][]byte{"config.yaml": release.Manifest()}
	switch {
	case reconcileCtx.IsShootDeployment:
		if err := managedresources.CreateForShoot(ctx, a.client, reconcileCtx.Namespace, constants.ManagedResourceNameShoot, constants.ExtensionServiceName, false, data); err != nil {
			return fmt.Errorf("could not create managed resource for the shoot: %w", err)
		}
	case reconcileCtx.IsSeedDeployment:
		// Resources must be provisioned in the same cluster (garden, seed).
		if err := managedresources.CreateForSeed(ctx, a.client, reconcileCtx.Namespace, constants.ManagedResourceNameShoot, false, data); err != nil {
			return fmt.Errorf("could not create managed resource for the seed: %w", err)
		}
	}
	return nil
}

func (a *actuator) createSeedResources(ctx context.Context, log logr.Logger, namespace string) error {
	// The SOC stack has no seed-side resources, and a class "seed" ManagedResource needs a
	// seed-class gardener-resource-manager to reconcile it, which this landscape does not
	// run. Creating it would leave an object nothing serves and a health check that can
	// never pass. Remove one if an earlier version left it behind.
	log.Info("no seed resources for this extension; ensuring none are left over", "namespace", namespace)
	return managedresources.DeleteForSeed(ctx, a.client, namespace, constants.ManagedResourceNameSeed)
}

// Delete the Extension resource.
func (a *actuator) Delete(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	namespace := ex.GetNamespace()
	if err := a.deleteShootResources(ctx, log, namespace, ex); err != nil {
		return fmt.Errorf("error deleting the SOC stack from %s: %w", namespace, err)
	}
	return a.createSeedResources(ctx, log, namespace)
}

// ForceDelete the Extension resource.
func (a *actuator) ForceDelete(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	return a.Delete(ctx, log, ex)
}

func (a *actuator) deleteShootResources(ctx context.Context, log logr.Logger, namespace string, ex *extensionsv1alpha1.Extension) error {
	log.Info(fmt.Sprintf("Deleting managed resource %s/%s", namespace, constants.ManagedResourceNameShoot))
	switch {
	case isShootDeployment(ex):
		if err := managedresources.DeleteForShoot(ctx, a.client, namespace, constants.ManagedResourceNameShoot); err != nil {
			return err
		}
	case isSeedDeployment(ex):
		if err := managedresources.DeleteForSeed(ctx, a.client, namespace, constants.ManagedResourceNameShoot); err != nil {
			return err
		}
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := managedresources.WaitUntilDeleted(timeoutCtx, a.client, namespace, constants.ManagedResourceNameShoot); err != nil {
		return err
	}
	log.Info(fmt.Sprintf("Successfully deleted managed resource %s/%s", namespace, constants.ManagedResourceNameShoot))
	return nil
}

// Restore the Extension resource.
func (a *actuator) Restore(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	return a.Reconcile(ctx, log, ex)
}

// Migrate the Extension resource.
func (a *actuator) Migrate(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	// Keep the objects so they are not deleted from the shoot during the migration.
	if err := managedresources.SetKeepObjects(ctx, a.client, ex.GetNamespace(), constants.ManagedResourceNameShoot, true); err != nil {
		return err
	}
	return a.Delete(ctx, log, ex)
}

func isShootDeployment(ex *extensionsv1alpha1.Extension) bool {
	return extensionsv1alpha1helper.GetExtensionClassOrDefault(ex.Spec.Class) == extensionsv1alpha1.ExtensionClassShoot
}

func isSeedDeployment(ex *extensionsv1alpha1.Extension) bool {
	return extensionsv1alpha1helper.GetExtensionClassOrDefault(ex.Spec.Class) == extensionsv1alpha1.ExtensionClassSeed
}

func isGardenDeployment(ex *extensionsv1alpha1.Extension) bool {
	return extensionsv1alpha1helper.GetExtensionClassOrDefault(ex.Spec.Class) == extensionsv1alpha1.ExtensionClassGarden
}

func getLocalClusterK8sVersion(cfg *rest.Config) (string, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("cannot get discovery client for local cluster %v", err)
	}
	v, err := discoveryClient.ServerVersion()
	if err != nil {
		return "", fmt.Errorf("cannot get kubernetes version of local cluster %v", err)
	}
	return v.Major + "." + v.Minor, nil
}
