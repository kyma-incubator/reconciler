package serverless

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const AggregateErrorFormat = "failed running %d action: %v, err: %w"

type ActionAggregate struct {
	actions []service.Action
}

func NewInstallActionAggregate(actions ...service.Action) *ActionAggregate {
	return &ActionAggregate{actions: actions}
}

func (a *ActionAggregate) Run(svcCtx *service.ActionContext) error {
	for i := 0; i < len(a.actions); i++ {
		err := a.actions[i].Run(svcCtx)
		if err != nil {
			return fmt.Errorf(AggregateErrorFormat, i, a.actions[i], err)
		}
	}
	return service.NewInstall(svcCtx.Logger).Invoke(svcCtx.Context, svcCtx.ChartProvider, svcCtx.Task, svcCtx.KubeClient)
}
