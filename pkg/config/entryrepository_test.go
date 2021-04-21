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

	t.Run("Validate data model", func(t *testing.T) {
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

}
