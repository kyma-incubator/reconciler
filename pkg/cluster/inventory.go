package cluster

import (
	"github.com/kyma-incubator/reconciler/pkg/repository"
)

type Cluster struct {
	Name       string
	Properties map[string]string
}

type Inventory struct {
	*repository.Repository
}

func (ci *Inventory) Get(cluster string) *Cluster {
	panic("not implemented yet")
}

func (ci *Inventory) Add(cluster *Cluster) error {
	panic("not implemented yet")
}

func (ci *Inventory) Delete(cluster string) error {
	panic("not implemented yet")
}
