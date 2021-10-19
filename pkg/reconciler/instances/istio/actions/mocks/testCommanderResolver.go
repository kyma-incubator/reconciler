package mock

import "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"

type TestCommanderResolver struct {
	err   error
	cmder istioctl.Commander
}

func (tcr TestCommanderResolver) GetCommander(version istioctl.Version) (istioctl.Commander, error) {
	if tcr.err != nil {
		return nil, tcr.err
	} else {
		return tcr.cmder, nil
	}
}
