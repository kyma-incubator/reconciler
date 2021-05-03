package config

import (
	"fmt"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/stretchr/testify/require"
)

func newRepo(t *testing.T) *ConfigEntryRepository {
	ceRepo, err := NewConfigEntryRepository(&db.PostgresConnectionFactory{
		Host:     "localhost",
		Port:     5432,
		Database: "kyma",
		User:     "kyma",
		Password: "kyma",
		Debug:    true,
	})
	require.NoError(t, err)
	return ceRepo
}
func TestEntryRepositoryKeys(t *testing.T) {
	var err error
	ceRepo := newRepo(t)

	//add test data
	keyID := fmt.Sprintf("testKey%d", time.Now().UnixNano())
	keyVersions := []int64{}

	t.Run("Validate entity and create test data", func(t *testing.T) {
		keyEntity := &KeyEntity{}
		_, err = ceRepo.CreateKey(keyEntity)
		require.True(t, db.IsIncompleteEntityError(err))

		keyEntity.Key = keyID
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
		entities, err := ceRepo.KeyHistory(keyID)
		require.NoError(t, err)
		require.Equal(t, 3, len(entities))
		//ordered by version ASC:
		require.True(t, entities[0].Version < entities[1].Version && entities[1].Version < entities[2].Version)
	})

	t.Run("Get latest keys", func(t *testing.T) {
		entity, err := ceRepo.LatestKey(keyID)
		require.NoError(t, err)
		require.Equal(t, keyVersions[len(keyVersions)-1], entity.Version)
	})

	t.Run("Get key", func(t *testing.T) {
		entity, err := ceRepo.Key(keyID, keyVersions[1])
		require.NoError(t, err)
		require.Equal(t, keyVersions[1], entity.Version)
	})

	t.Run("Delete key", func(t *testing.T) {
		entity, err := ceRepo.Key(keyID, keyVersions[1])
		require.NoError(t, err)
		err = ceRepo.DeleteKey(entity)
		require.NoError(t, err)
		entities, err := ceRepo.KeyHistory(keyID)
		require.NoError(t, err)
		require.Equal(t, 2, len(entities)) //ensure just 2 entities were left
	})
}

func TestEntryRepositoryValues(t *testing.T) {
	var err error
	ceRepo := newRepo(t)

	keyEntity, err := ceRepo.CreateKey(&KeyEntity{
		Key:      fmt.Sprintf("testKey%d", time.Now().UnixNano()),
		DataType: String,
		Username: "testUsername",
	})
	require.NoError(t, err)
	require.NotEmpty(t, keyEntity)
	bucketNames := []string{"testBucket", "anotherTestBucket"}

	valueVersions := []int64{}
	t.Run("Validate entity and create test data", func(t *testing.T) {
		valueEntity := &ValueEntity{}
		_, err = ceRepo.CreateValue(valueEntity)
		require.True(t, db.IsIncompleteEntityError(err))

		valueEntity.Bucket = bucketNames[0]
		_, err = ceRepo.CreateValue(valueEntity)
		require.True(t, db.IsIncompleteEntityError(err))

		valueEntity.Key = keyEntity.Key
		_, err = ceRepo.CreateValue(valueEntity)
		require.True(t, db.IsIncompleteEntityError(err))

		valueEntity.KeyVersion = keyEntity.Version
		_, err = ceRepo.CreateValue(valueEntity)
		require.True(t, db.IsIncompleteEntityError(err))

		valueEntity.Username = "testUsername"
		_, err = ceRepo.CreateValue(valueEntity)
		require.True(t, db.IsIncompleteEntityError(err))

		for _, value := range []string{"test value 1", "test value 2", "test value 3"} {
			valueEntity.Value = value
			valueEntity, err := ceRepo.CreateValue(valueEntity)
			require.NoError(t, err)
			valueVersions = append(valueVersions, valueEntity.Version)
		}
	})

	t.Run("Get value history", func(t *testing.T) {
		entities, err := ceRepo.ValueHistory(bucketNames[0], keyEntity.Key)
		require.NoError(t, err)
		require.Equal(t, 3, len(entities))
		//ordered by version ASC:
		require.True(t, entities[0].Version < entities[1].Version && entities[1].Version < entities[2].Version)
	})

	t.Run("Get latest value", func(t *testing.T) {
		entity, err := ceRepo.LatestValue(bucketNames[0], keyEntity.Key)
		require.NoError(t, err)
		require.Equal(t, valueVersions[len(valueVersions)-1], entity.Version)
	})

	t.Run("Get value", func(t *testing.T) {
		entity, err := ceRepo.Value(bucketNames[0], keyEntity.Key, valueVersions[1])
		require.NoError(t, err)
		require.Equal(t, valueVersions[1], entity.Version)
	})

	t.Run("Get values in bucket", func(t *testing.T) {
		entities, err := ceRepo.Values(bucketNames[0])
		require.NoError(t, err)
		require.Equal(t, 1, len(entities))
		require.Equal(t, valueVersions[len(valueVersions)-1], entities[0].Version)
	})

	t.Run("Get buckets", func(t *testing.T) {
		valueEntity := &ValueEntity{
			Key:        keyEntity.Key,
			KeyVersion: keyEntity.Version,
			Username:   "xyz123",
			Bucket:     "anotherTestBucket",
			Value:      "another value",
		}
		_, err := ceRepo.CreateValue(valueEntity)
		require.NoError(t, err)
		buckets, err := ceRepo.Buckets()
		require.NoError(t, err)

		//at least the buckets created during this test run have to exist:
		require.True(t, len(buckets) >= 2)

		//check that expected bucket were returned
		bucketNamesGot := []string{}
		for _, bucket := range buckets {
			bucketNamesGot = append(bucketNamesGot, bucket.Bucket)
		}
		for _, bucketNameExpected := range bucketNames {
			require.Contains(t, bucketNamesGot, bucketNameExpected)
		}

	})

	t.Run("Delete bucket", func(t *testing.T) {
		for _, bucketName := range bucketNames {
			err := ceRepo.DeleteBucket(&BucketEntity{
				Bucket: bucketName,
			})
			require.NoError(t, err)
		}
		buckets, err := ceRepo.Buckets()
		require.NoError(t, err)
		bucketNamesGot := []string{}
		for _, bucket := range buckets {
			bucketNamesGot = append(bucketNamesGot, bucket.Bucket)
		}
		for _, bucketNameNotExpected := range bucketNames {
			require.NotContains(t, bucketNamesGot, bucketNameNotExpected)
		}
	})
}
