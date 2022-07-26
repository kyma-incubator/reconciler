package mock

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/helpers"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"
)

type TestCommanderResolver struct {
	err   error
	cmder istioctl.Commander
}

func (tcr TestCommanderResolver) GetCommander(_ helpers.HelperVersion) (istioctl.Commander, error) {
	if tcr.err != nil {
		return nil, tcr.err
	}
	return tcr.cmder, nil
}
