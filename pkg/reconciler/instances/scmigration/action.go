package scmigration

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

type reconcileAction struct {
	// TODO add prometheus metrics
}

func (a *reconcileAction) Run(ac *service.ActionContext) error {
	if _, err := ac.KubeClient.Clientset(); err != nil {
		ac.Logger.Errorf("Failed to retrieve native Kubernetes GO client")
	}

	ac.Logger.Infof("Action 'reconcileAction' executed (passed version was '%s')", ac.Task.Version)
	m, err := newMigrator(ac)
	if err != nil {
		return err
	}
	return m.migrateBTPOperator()
}
