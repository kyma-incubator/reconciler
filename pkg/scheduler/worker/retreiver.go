package worker

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

type ClusterStateRetriever interface {
	Get(op *model.OperationEntity) (*cluster.State, error)
}

type InventoryRetriever struct {
	Inventory cluster.Inventory
}

func (r *InventoryRetriever) Get(op *model.OperationEntity) (*cluster.State, error) {
	return r.Inventory.Get(op.Cluster, op.ClusterConfig)
}

type PassThroughRetriever struct {
	State *cluster.State
}

func (r *PassThroughRetriever) Get(_ *model.OperationEntity) (*cluster.State, error) {
	return r.State, nil
}
