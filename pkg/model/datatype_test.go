package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDataType(t *testing.T) {
	t.Run("New data type", func(t *testing.T) {
		_, err := NewDataType("test")
		require.Error(t, err)

		dt, err := NewDataType("integer")
		require.NoError(t, err)
		require.Equal(t, dt, Integer)

		dt, err = NewDataType("BooLEan")
		require.NoError(t, err)
		require.Equal(t, dt, Boolean)

		dt, err = NewDataType("string")
		require.NoError(t, err)
		require.Equal(t, dt, String)
	})

}
