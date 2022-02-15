package invoker

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestInvoker(t *testing.T) {

	params := Params{
		ComponentToReconcile: &keb.Component{
			URL:       "",
			Component: "",
			Configuration: []keb.Configuration{
				{
					Key:    "repo.token.namespace",
					Secret: false,
					Value:  nil,
				},
			},
			Namespace: "",
			Version:   "",
		},
		ComponentsReady:     nil,
		ClusterState:        clusterStateMock,
		SchedulingID:        "",
		CorrelationID:       "",
		MaxOperationRetries: 0,
		Type:                model.OperationTypeDelete,
	}

	task := params.newTask()
	assert.Equal(t, "", task.Repository.TokenNamespace, "Should parse repo token namespace correctly")
	assert.Equal(t, model.OperationTypeDelete, task.Type, "Task type should equal operation type")
}
