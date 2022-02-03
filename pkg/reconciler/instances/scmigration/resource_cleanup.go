package scmigration

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	HelmBrokerComponent           = "helm-broker"
	ServiceCatalogAddonsComponent = "service-catalog-addons"
	ServiceCatalogComponent       = "service-catalog"
	ServiceManagerProxyComponent  = "service-manager-proxy"
)

var (
	resources = map[string][]*unstructured.Unstructured{
		HelmBrokerComponent:           hbResources,
		ServiceCatalogAddonsComponent: svcatAddonsResources,
		ServiceCatalogComponent:       svcatResources,
		ServiceManagerProxyComponent:  smProxyResources,
	}

	smProxyResources = []*unstructured.Unstructured{
		&serviceManagerProxyUnstructuredServiceAccount,
		&serviceManagerProxyRegsecretUnstructuredSecret,
		&serviceManagerProxyConfigUnstructuredConfigMap,
		&serviceManagerProxyUnstructuredClusterRole,
		&serviceManagerProxyUnstructuredClusterRoleBinding,
		&serviceManagerProxyRegsecretviewerUnstructuredRole,
		&serviceManagerProxyUnstructuredRoleBinding,
		&serviceManagerProxyUnstructuredService,
		&serviceManagerProxyUnstructuredDeployment,
		&serviceManagerProxyUnstructuredServiceAccount,
		&serviceManagerProxyRegsecretUnstructuredSecret,
		&serviceManagerProxyConfigUnstructuredConfigMap,
		&serviceManagerProxyUnstructuredClusterRole,
		&serviceManagerProxyUnstructuredClusterRoleBinding,
		&serviceManagerProxyRegsecretviewerUnstructuredRole,
		&serviceManagerProxyUnstructuredRoleBinding,
		&serviceManagerProxyUnstructuredService,
		&serviceManagerProxyUnstructuredDeployment,
	}

	svcatResources = []*unstructured.Unstructured{
		&serviceCatalogControllerManagerUnstructuredServiceAccount,
		&serviceCatalogWebhookUnstructuredServiceAccount,
		&serviceCatalogTestsUnstructuredServiceAccount,
		&serviceCatalogCatalogWebhookCertUnstructuredSecret,
		&serviceCatalogDashboardUnstructuredConfigMap,
		&servicecatalogK8SIocontrollerManagerUnstructuredClusterRole,
		&servicecatalogK8SIoserviceCatalogReadinessUnstructuredClusterRole,
		&servicecatalogK8SIowebhookUnstructuredClusterRole,
		&serviceCatalogTestsUnstructuredClusterRole,
		&servicecatalogK8SIocontrollerManagerUnstructuredClusterRoleBinding,
		&servicecatalogK8SIoserviceCatalogReadinessUnstructuredClusterRoleBinding,
		&servicecatalogK8SIowebhookUnstructuredClusterRoleBinding,
		&serviceCatalogTestsUnstructuredClusterRoleBinding,
		&servicecatalogK8SIoclusterInfoConfigmapUnstructuredRole,
		&servicecatalogK8SIoleaderLockingControllerManagerUnstructuredRole,
		&serviceCatalogControllerManagerClusterInfoUnstructuredRoleBinding,
		&serviceCatalogControllerManagerLeaderElectionUnstructuredRoleBinding,
		&serviceCatalogCatalogControllerManagerUnstructuredService,
		&serviceCatalogCatalogWebhookUnstructuredService,
		&serviceCatalogCatalogControllerManagerUnstructuredDeployment,
		&serviceCatalogCatalogWebhookUnstructuredDeployment,
		&servicecatalogUnstructuredBackendModule,
		&serviceCatalogCatalogWebhookUnstructuredMutatingWebhookConfiguration,
		&serviceCatalogCatalogControllerManagerUnstructuredPeerAuthentication,
		&serviceCatalogCatalogWebhookUnstructuredPeerAuthentication,
		&serviceCatalogCatalogControllerManagerUnstructuredServiceMonitor,
		&serviceCatalogUnstructuredTestDefinition,
		&serviceCatalogCatalogValidatingWebhookUnstructuredValidatingWebhookConfiguration,
	}

	svcatAddonsResources = []*unstructured.Unstructured{
		&serviceCatalogAddonsServiceBindingUsageControllerCleanupUnstructuredJob,
		&serviceCatalogAddonsServiceCatalogUiUnstructuredPodSecurityPolicy,
		&serviceCatalogAddonsServiceBindingUsageControllerUnstructuredServiceAccount,
		&serviceCatalogAddonsServiceCatalogUiUnstructuredServiceAccount,
		&serviceBindingUsageControllerProcessSbuSpecUnstructuredConfigMap,
		&serviceBindingUsageControllerDashboardUnstructuredConfigMap,
		&serviceCatalogUiUnstructuredConfigMap,
		&serviceCatalogAddonsServiceBindingUsageControllerUnstructuredClusterRole,
		&serviceCatalogAddonsServiceBindingUsageControllerUnstructuredClusterRoleBinding,
		&serviceCatalogAddonsServiceCatalogUiUnstructuredRole,
		&serviceCatalogAddonsServiceCatalogUiUnstructuredRoleBinding,
		&serviceCatalogAddonsServiceBindingUsageControllerUnstructuredService,
		&serviceCatalogAddonsServiceCatalogUiUnstructuredService,
		&serviceCatalogAddonsServiceBindingUsageControllerUnstructuredDeployment,
		&serviceCatalogAddonsServiceCatalogUiUnstructuredDeployment,
		&serviceCatalogAddonsServiceCatalogUiUnstructuredDestinationRule,
		&serviceCatalogAddonsServiceBindingUsageControllerUnstructuredPeerAuthentication,
		&serviceCatalogAddonsServiceBindingUsageControllerUnstructuredServiceMonitor,
		&deploymentUnstructuredUsageKind,
		&serviceCatalogAddonsServiceCatalogUiCatalogUnstructuredVirtualService,
	}

	hbResources = []*unstructured.Unstructured{
		&helmBrokerCleanupUnstructuredJob,
		&helmBrokerAddonsUiUnstructuredPodSecurityPolicy,
		&helmBrokerAddonsUiUnstructuredServiceAccount,
		&helmBrokerEtcdStatefulEtcdCertsUnstructuredServiceAccount,
		&helmBrokerUnstructuredServiceAccount,
		&helmSecretUnstructuredSecret,
		&helmBrokerWebhookCertUnstructuredSecret,
		&addonsUiUnstructuredConfigMap,
		&helmBrokerDashboardUnstructuredConfigMap,
		&helmConfigMapUnstructuredConfigMap,
		&sshCfgUnstructuredConfigMap,
		&helmBrokerEtcdStatefulEtcdCertsUnstructuredClusterRole,
		&helmBrokerH3UnstructuredClusterRole,
		&helmBrokerEtcdStatefulEtcdCertsUnstructuredClusterRoleBinding,
		&helmBrokerH3UnstructuredClusterRoleBinding,
		&helmBrokerAddonsUiUnstructuredRole,
		&helmBrokerAddonsUiUnstructuredRoleBinding,
		&helmBrokerAddonsUiUnstructuredService,
		&helmBrokerEtcdStatefulUnstructuredService,
		&helmBrokerEtcdStatefulClientUnstructuredService,
		&helmBrokerMetricsUnstructuredService,
		&addonControllerMetricsUnstructuredService,
		&helmBrokerUnstructuredService,
		&helmBrokerWebhookUnstructuredService,
		&helmBrokerAddonsUiUnstructuredDeployment,
		&helmBrokerUnstructuredDeployment,
		&helmBrokerWebhookUnstructuredDeployment,
		&helmBrokerEtcdStatefulUnstructuredStatefulSet,
		&helmBrokerUnstructuredAuthorizationPolicy,
		&helmReposUrlsUnstructuredClusterAddonsConfiguration,
		&addonsclustermicrofrontendUnstructuredClusterMicroFrontend,
		&addonsmicrofrontendUnstructuredClusterMicroFrontend,
		&helmBrokerAddonsUiUnstructuredDestinationRule,
		&helmBrokerEtcdStatefulClientUnstructuredDestinationRule,
		&helmBrokerMutatingWebhookUnstructuredMutatingWebhookConfiguration,
		&helmBrokerUnstructuredPeerAuthentication,
		&helmBrokerEtcdStatefulUnstructuredServiceMonitor,
		&helmBrokerUnstructuredServiceMonitor,
		&helmBrokerAddonControllerUnstructuredServiceMonitor,
		&helmBrokerAddonsUiUnstructuredVirtualService,
	}
)
