package connectivityproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMap(t *testing.T) {
	var testMap Map = map[string]interface{}{
		"key-0": "value-0",
		"key-2": "value-1",
		"key-3": map[string]interface{}{
			"key-4": map[string]interface{}{
				"key-5": "value-5",
			},
		},
		"key-6": nil,
	}

	t.Run("Should get value from unstructured", func(t *testing.T) {
		value := testMap.getValue("key-0")
		require.Equal(t, "value-0", value)

		value = testMap.getValue("key-3", "key-4", "key-5")
		require.Equal(t, "value-5", value)
	})

	t.Run("Should return nil if not existing", func(t *testing.T) {
		value := testMap.getValue("key-6")
		require.Equal(t, nil, value)
	})

	t.Run("Should return if not existing", func(t *testing.T) {
		value := testMap.getValue("key-6", "key-x")
		require.Equal(t, nil, value)
	})

	t.Run("Should return error if looking in an empty/nil map", func(t *testing.T) {
		nilMap := make(map[string]interface{})
		var nilMapCasted Map = nilMap
		value := nilMapCasted.getValue("key-6", "key-x")
		require.Equal(t, nil, value)
	})

	t.Run("Should return extract secret name", func(t *testing.T) {
		mapWithSecretName := map[string]interface{}{
			"spec": map[string]interface{}{
				"secretName": "test-secret-name",
			},
		}
		var nilMapCasted Map = mapWithSecretName
		value, err := nilMapCasted.getSecretName()
		require.NoError(t, err)
		require.Equal(t, "test-secret-name", value)
	})
}
