package cluster

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"time"
)

type StatusChange struct {
	Status   *model.ClusterStatusEntity
	Duration time.Duration
}

func (s *StatusChange) String() string {
	return fmt.Sprintf("StatusChange [Status=%s,Duration=%s]", s.Status.Status, s.Duration)
}
