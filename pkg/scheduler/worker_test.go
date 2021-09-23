package scheduler

import (
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestWorker(t *testing.T) {
	t.Parallel()

	t.Run("Should parse configs", func(t *testing.T) {
		configsMap := keb.Component{

			Configuration: []keb.Configuration{
				{
					Key:   "test1",
					Value: "value1",
				},
				{
					Key:   "test2",
					Value: "value2",
				},
			},
		}.ConfigurationAsMap()

		assert.Equal(t, map[string]interface{}{
			"test1": "value1",
			"test2": "value2",
		}, configsMap)
	})
}
