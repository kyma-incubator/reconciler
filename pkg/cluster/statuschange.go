package cluster

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

type StatusChange struct {
	Status   *model.Status
	Duration string
}

func (s *StatusChange) String() string {
	return fmt.Sprintf("StatusChange [Status=%s,Duration=%s]",
		s.Status, s.Duration)
}
