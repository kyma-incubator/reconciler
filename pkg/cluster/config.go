package cluster

import (
	"github.com/kyma-incubator/reconciler/pkg/kv"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

var defaultBucketPrecedence = []string{model.DefaultBucket, "profile", "customer", "cluster", "feature"}

type Configuration struct {
	Cluster      string
	bucketMerger *bucketMerger
}

type Configurer struct {
	kvRepository *kv.Repository
	inventory    *Inventory
}
