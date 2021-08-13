package scheduler

import (
	"encoding/json"

	"github.com/kyma-incubator/reconciler/pkg/keb"
)

func fixUgliness(components []keb.Components) string {
	result, _ := json.Marshal(components)
	return string(result)
}
