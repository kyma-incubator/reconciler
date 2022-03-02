package provisioning

import (
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/provisioning/util"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

type ProvisioningAction struct {
	name        string
	kubeconfig  string
	provisioner *asyncProvisioner
}

func (a *ProvisioningAction) Run(ctx *service.ActionContext) error {
	// this is not an instance of the provisioner reconciler deployment
	if a.provisioner == nil {
		ctx.Logger.Infof("Action '%s' skipped (passed version was '%s')", a.name, ctx.Task.Version)
		return nil
	}

	// get configuration
	if ctx.Task.Type == model.OperationTypeReconcile {

		kebConfig := a.getKEBConfig(ctx)

		err := a.provisioner.ProvisionCluster(ctx.Context, *kebConfig, ctx.Task.Metadata.GlobalAccountID, &ctx.Task.Metadata.SubAccountID, a.getClusterID(ctx), a.getOperationID(ctx))

		if err != nil {
			ctx.Logger.Errorf("Action '%s' failed: %s", a.name, err)
		} else {
			ctx.Logger.Infof("Action '%s' executed (passed version was '%s')", a.name, ctx.Task.Version)
		}
	} else if ctx.Task.Type == model.OperationTypeDelete {
		ctx.Logger.Info("Action of Kyma cluster removal is not net implemented yet")
	}

	return nil
}

func (a *ProvisioningAction) getClusterID(ctx *service.ActionContext) string {
	return "my_little_cluster_id"
}

func (a *ProvisioningAction) getOperationID(ctx *service.ActionContext) string {
	return "some_id"
}

// put in some dummy configuration
func (a *ProvisioningAction) getKEBConfig(ctx *service.ActionContext) *keb.GardenerConfig {
	// TODO: read keb.Config from the context

	return &keb.GardenerConfig{
		AllowPrivilegedContainers:           false,
		AutoScalerMax:                       4,
		AutoScalerMin:                       1,
		DiskType:                            util.StringPtr("standard"),
		EnableKubernetesVersionAutoUpdate:   false,
		EnableMachineImageVersionAutoUpdate: false,
		KubernetesVersion:                   "1.19.15",
		MachineType:                         "Standard_D4_v3",
		MaxSurge:                            4,
		MaxUnavailable:                      1,
		Name:                                "test-recon",
		ProjectName:                         "frog-dev",
		Provider:                            "azure",
		// TODO: ProviderSpecificConfig must be obligatory
		ProviderSpecificConfig: &keb.ProviderSpecificConfig{
			Azure: &keb.AzureProviderConfig{
				VnetCidr: "10.250.0.0/19",
			},
		},
		Region: "westeurope",
		// TODO: shouldn't Seed be optional?
		TargetSecret: "my-azure-secret",
		// TODO: shouldn't VolumeSizeGB be obligatory?
		VolumeSizeGB: util.IntPtr(50),
		WorkerCidr:   "10.250.0.0/19",
	}
}

