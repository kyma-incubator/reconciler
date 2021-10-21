package invoker

import (
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestInvoker(t *testing.T) {
	t.Run("Should parse repo token namespace correctly", func(t *testing.T) {
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
			ComponentsReady: nil,
			ClusterState:    clusterStateMock,
			SchedulingID:    "",
			CorrelationID:   "",
		}

		model := params.newReconciliationModel()
		assert.Equal(t, "", model.Repository.TokenNamespace)
	})
}
