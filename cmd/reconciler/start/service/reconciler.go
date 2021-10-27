package cmd

import (
	"context"

	reconCli "github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

func StartComponentReconciler(ctx context.Context, o *reconCli.Options, reconcilerName string) (*service.WorkerPool, error) {
	recon, err := reconCli.NewComponentReconciler(o, reconcilerName)
	if err != nil {
		return nil, err
	}

	o.Logger().Infof("Starting component reconciler '%s'", reconcilerName)
	return recon.StartRemote(ctx)
}
