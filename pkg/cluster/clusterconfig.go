package cluster

import (
	"github.com/kyma-incubator/reconciler/pkg/kv"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

var defaultMergeSequence = []string{model.DefaultBucket, "profile", "customer", "cluster", "feature"}

type ClusterConfiguration struct {
	kvRepository *kv.KeyValueRepository
	inventory    *Inventory
}
