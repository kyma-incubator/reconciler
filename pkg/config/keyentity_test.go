package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyEntity(t *testing.T) {
	t.Run("Validate valid string", func(t *testing.T) {
		key := &KeyEntity{
			Key:       "Mock",
			DataType:  String,
			Validator: `len(it) > 5`,
		}
		err := key.Validate("thisIsOk")
		require.NoError(t, err)
	})

	t.Run("Validate invalid string", func(t *testing.T) {
		key := &KeyEntity{
			Key:       "Mock",
			DataType:  String,
			Validator: `len(it) > 5`,
		}
		err := key.Validate("abc")
		require.Error(t, err)
		require.True(t, IsInvalidValueError(err))
	})

	t.Run("Validate valid integer", func(t *testing.T) {
		key := &KeyEntity{
			Key:       "Mock",
			DataType:  Integer,
			Validator: `it - 123456788 == 1`,
		}
		err := key.Validate("123456789")
		require.NoError(t, err)
	})

	t.Run("Validate invalid integer", func(t *testing.T) {
		key := &KeyEntity{
			Key:       "Mock",
			DataType:  Integer,
			Validator: `it > 0`,
		}
		err := key.Validate("abc")
		require.Error(t, err)
		require.False(t, IsInvalidValueError(err)) //is code error
	})

	t.Run("Validate valid boolean", func(t *testing.T) {
		key := &KeyEntity{
			Key:       "Mock",
			DataType:  Boolean,
			Validator: `it == true`,
		}
		err := key.Validate("TruE")
		require.NoError(t, err)
	})

	t.Run("Validate invalid boolean", func(t *testing.T) {
		key := &KeyEntity{
			Key:       "Mock",
			DataType:  Boolean,
			Validator: `it == true`,
		}
		err := key.Validate("TRUEE")
		require.Error(t, err)
		require.False(t, IsInvalidValueError(err)) //is code error
	})
}
