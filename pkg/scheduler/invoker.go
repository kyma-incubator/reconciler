package scheduler

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
)

type InvokeParams struct {
	ComponentToReconcile *keb.Component
	ComponentsReady      []string
	ClusterState         cluster.State
	SchedulingID         string
	CorrelationID        string
	ReconcilerURL        string
	InstallCRD           bool
}

type ReconcilerInvoker interface {
	Invoke(params *InvokeParams) error
}
