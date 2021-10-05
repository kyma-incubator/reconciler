package keb

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestContract(t *testing.T) {
	t.Run("Configuration as map", func(t *testing.T) {
		comp := &Component{
			Configuration: []Configuration{
				{
					Key:   "test1",
					Value: "value1",
				},
				{
					Key:   "test2",
					Value: "value2",
				},
			},
		}

		require.Equal(t, map[string]interface{}{
			"test1": "value1",
			"test2": "value2",
		}, comp.ConfigurationAsMap())
	})
}
