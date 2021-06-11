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
	return nil
}

func (ci *Inventory) Add(cluster *Cluster) error {
	return nil
}

func (ci *Inventory) Delete(cluster string) error {
	return nil
}
