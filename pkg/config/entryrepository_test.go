package config

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/stretchr/testify/require"
)

func TestEntryRepository(t *testing.T) {
	ceRepo, err := NewConfigEntryRepository(&db.PostgresConnectionFactory{
		Host:     "localhost",
		User:     "kyma",
		Password: "kyma",
		Database: "kyma",
		Debug:    true,
	})
	require.NoError(t, err)

	t.Run("Validate and crate entity", func(t *testing.T) {
		keyEntity := &KeyEntity{}
		_, err = ceRepo.CreateKey(keyEntity)
		require.True(t, db.IsIncompleteEntityError(err))

		keyEntity.Key = "testKey"
		_, err = ceRepo.CreateKey(keyEntity)
		require.True(t, db.IsIncompleteEntityError(err))

		keyEntity.Username = "testUser"
		_, err = ceRepo.CreateKey(keyEntity)
		require.True(t, db.IsIncompleteEntityError(err))

		keyEntity.DataType = String
		entity, err := ceRepo.CreateKey(keyEntity)
		require.NoError(t, err)
		require.NotEmpty(t, entity)
	})

	t.Run("Get all keys", func(t *testing.T) {
		var err error
		_, err = ceRepo.CreateKey(&KeyEntity{
			Key:      "testKey1",
			Username: "abc",
			DataType: String,
		})
		require.NoError(t, err)
		_, err = ceRepo.CreateKey(&KeyEntity{
			Key:      "testKey1",
			Username: "abc",
			DataType: Integer,
		})
		require.NoError(t, err)
		entities, err := ceRepo.GetKeys("testKey1")
		require.NoError(t, err)
		require.NotEmpty(t, entities)
	})

}
