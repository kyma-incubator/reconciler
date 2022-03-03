package provisioning

import (
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
)

type ProvisioningAction struct {
	name       string
	kubeconfig string
}

func (a *ProvisioningAction) Run(ctx *service.ActionContext) error {

	if ctx.Task.Type == model.OperationTypeReconcile {
		gardenerConfig, err := a.getGardenerConfig(ctx)
		if err != nil {
			return err
		}
		provisioner, err := createProvisioner(ctx.Logger)
		if err != nil {
			return err
		}

		err = provisioner.ProvisionOrUpgrade(ctx.Context, gardenerConfig, ctx.Task.Metadata.GlobalAccountID, &ctx.Task.Metadata.SubAccountID, a.getClusterID(ctx), a.getOperationID(ctx))

		if err != nil {
			ctx.Logger.Errorf("Action '%s' failed: %s", a.name, err)
		} else {
			ctx.Logger.Infof("Action '%s' executed (passed version was '%s')", a.name, ctx.Task.Version)
		}

		return err
	} else if ctx.Task.Type == model.OperationTypeDelete {
		ctx.Logger.Info("Action of Kyma cluster removal is not net implemented yet")
	}

	return nil
}

func (a *ProvisioningAction) getClusterID(ctx *service.ActionContext) string {
	// TODO: needs to adjust the ProvisionOnUpgrade interface. At this point Task contains CorrelationID only.
	return ""
}

func (a *ProvisioningAction) getOperationID(ctx *service.ActionContext) string {
	// TODO: needs to adjust the ProvisionOnUpgrade interface. At this point Task contains CorrelationID only.
	return ctx.Task.CorrelationID
}

func (a *ProvisioningAction) getGardenerConfig(ctx *service.ActionContext) (keb.GardenerConfig, error) {
	config, ok := (ctx.Task.Configuration["gardenerConfig"]).(keb.GardenerConfig)
	if ok {
		return config, nil
	}

	return keb.GardenerConfig{}, errors.New("failed to get Gardener Config")
}
