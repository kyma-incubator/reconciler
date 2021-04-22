package config

import (
	"fmt"
	"testing"
	"time"

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

	//add test data
	keyId := fmt.Sprintf("testKey%d", time.Now().UnixNano())
	keyVersions := []int64{}

	t.Run("Validate entity and create test data", func(t *testing.T) {
		keyEntity := &KeyEntity{}
		_, err = ceRepo.CreateKey(keyEntity)
		require.True(t, db.IsIncompleteEntityError(err))

		keyEntity.Key = keyId
		_, err = ceRepo.CreateKey(keyEntity)
		require.True(t, db.IsIncompleteEntityError(err))

		keyEntity.Username = "abc"
		_, err = ceRepo.CreateKey(keyEntity)
		require.True(t, db.IsIncompleteEntityError(err))

		for _, dt := range []DataType{String, Boolean, Integer} {
			keyEntity.DataType = dt
			entity, err := ceRepo.CreateKey(keyEntity)
			require.NoError(t, err)
			keyVersions = append(keyVersions, entity.Version)
		}
	})

	t.Run("Get all keys", func(t *testing.T) {
		entities, err := ceRepo.GetKeys(keyId)
		require.NoError(t, err)
		require.Equal(t, 3, len(entities))
		//ordered by version ASC:
		require.True(t, entities[0].Version < entities[1].Version && entities[1].Version < entities[2].Version)
	})

	t.Run("Get latest keys", func(t *testing.T) {
		entity, err := ceRepo.GetLatestKey(keyId)
		require.NoError(t, err)
		require.Equal(t, keyVersions[len(keyVersions)-1], entity.Version)
	})

	t.Run("Get key", func(t *testing.T) {
		entity, err := ceRepo.GetKey(keyId, keyVersions[1])
		require.NoError(t, err)
		require.Equal(t, keyVersions[1], entity.Version)
	})
}
