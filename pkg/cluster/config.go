package cluster

import (
	"github.com/kyma-incubator/reconciler/pkg/kv"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

var defaultBucketPrecedence = []string{model.DefaultBucket, "profile", "customer", "cluster", "feature"}

type Configuration struct {
	cluster      string
	bucketMerger *bucketMerger
}

type Configurer struct {
	kvRepository *kv.Repository
	inventory    *Inventory
}

func (c *Configurer) Get(cluster string) *Configuration {
	panic("not implemented yet")
}
