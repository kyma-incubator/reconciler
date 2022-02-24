package scmigration

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

type reconcileAction struct {
	// TODO add prometheus metrics
}

func (a *reconcileAction) Run(ac *service.ActionContext) error {
	ac.Logger = ac.Logger.With("instanceID", ac.Task.Metadata.InstanceID)
	if _, err := ac.KubeClient.Clientset(); err != nil {
		ac.Logger.Errorf("Failed to retrieve native Kubernetes GO client")
	}
	ac.Logger.Infof("Action 'reconcileAction' executed (passed version was '%s')", ac.Task.Version)
	ac.Logger.Infof("Remove owner references from SBUs")
	cleaner, err := newSCRemovalClient([]byte(ac.KubeClient.Kubeconfig()))
	if err != nil {
		return err
	}
	if err := cleaner.prepareSBUsForRemoval(ac); err != nil {
		return err
	}
	ac.Logger.Infof("Executing SVCAT => BTP Operator CRD migration")
	m, err := newMigrator(ac)
	if err != nil {
		return err
	}
	if err := m.migrateBTPOperator(); err != nil {
		return err
	}

	ac.Logger.Infof("Remove service-catalog")
	if err := cleaner.removeRelease(ServiceCatalogComponent, ac); err != nil {
		return err
	}
	ac.Logger.Infof("Remove service-catalog-addons")
	if err := cleaner.removeRelease(ServiceCatalogAddonsComponent, ac); err != nil {
		return err
	}
	ac.Logger.Infof("Remove helm-broker")
	if err := cleaner.removeRelease(HelmBrokerComponent, ac); err != nil {
		return err
	}
	ac.Logger.Infof("Remove service-manager-proxy")
	if err := cleaner.removeRelease(ServiceManagerProxyComponent, ac); err != nil {
		return err
	}

	ac.Logger.Infof("Ensure service catalog is not running")
	if err := cleaner.ensureServiceCatalogNotRunning(ac); err != nil {
		return err
	}

	ac.Logger.Infof("Remove finalizers")
	if err := cleaner.prepareForRemoval(ac); err != nil {
		return err
	}

	ac.Logger.Infof("Delete resources")
	if err := cleaner.removeResources(ac); err != nil {
		return err
	}

	// NOTE: currently out of scope, CRDs will be removed in a later release
	//ac.Logger.Infof("Delete CRDs")
	//if err := cleaner.RemoveCRDs(); err != nil {
	//	return err
	//}
	return nil
}
