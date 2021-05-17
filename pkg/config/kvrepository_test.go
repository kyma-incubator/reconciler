package config

import (
	"fmt"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/stretchr/testify/require"
)

func TestKeyValueRepositoryKeys(t *testing.T) {
	var err error
	ceRepo := newKeyValueRepo(t)

	//add test data
	ts := time.Now().UnixNano()
	keyIDs := []string{fmt.Sprintf("testKey-%d", ts), fmt.Sprintf("testKey2-%d", ts)} //these are the IDs of the two create test keys
	key1Versions := []int64{}                                                         //contains the three versions of the first test key
	key2Versions := []int64{}                                                         //contains the three versions of the second test key

	t.Run("Validate entity and create first test key", func(t *testing.T) {
		keyEntity := &KeyEntity{}
		_, err = ceRepo.CreateKey(keyEntity)
		require.True(t, db.IsInvalidEntityError(err))

		keyEntity.Key = keyIDs[0]
		_, err = ceRepo.CreateKey(keyEntity)
		require.True(t, db.IsInvalidEntityError(err))

		keyEntity.Username = "abc"
		_, err = ceRepo.CreateKey(keyEntity)
		require.True(t, db.IsInvalidEntityError(err))

		//create first test key in 3 versions
		for _, dt := range []DataType{String, Boolean, Integer} {
			keyEntity.DataType = dt
			entity, err := ceRepo.CreateKey(keyEntity)
			require.NoError(t, err)
			key1Versions = append(key1Versions, entity.Version)
		}

		//create second key in 3 versions
		key2Entity := &KeyEntity{
			Key:      keyIDs[1],
			Username: "xyz",
		}
		for _, dt := range []DataType{String, Boolean, Integer} {
			key2Entity.DataType = dt
			_, err := ceRepo.CreateKey(key2Entity)
			require.NoError(t, err)
			key2Versions = append(key2Versions, key2Entity.Version)
		}
	})

	t.Run("Get keys", func(t *testing.T) {
		//at least 2 keys have to exist
		keyEntities, err := ceRepo.Keys()
		require.NoError(t, err)
		require.True(t, len(keyEntities) >= 2)

		//ensure that previously created test keys are part of the result
		keysByName := make(map[string]*KeyEntity, len(keyEntities))
		for _, keyEntity := range keyEntities {
			keysByName[keyEntity.Key] = keyEntity
		}
		for expectedKeyName, expectedVersion := range map[string]int64{keyIDs[0]: key1Versions[2], keyIDs[1]: key2Versions[2]} {
			key, ok := keysByName[expectedKeyName]
			require.True(t, ok)
			require.Equal(t, expectedVersion, key.Version)
		}
	})

	t.Run("Get key history", func(t *testing.T) {
		keyEntities, err := ceRepo.KeyHistory(keyIDs[0])
		require.NoError(t, err)
		require.Equal(t, 3, len(keyEntities))
		//ordered by version ASC:
		require.True(t, keyEntities[0].Version < keyEntities[1].Version && keyEntities[1].Version < keyEntities[2].Version)
	})

	t.Run("Get latest keys", func(t *testing.T) {
		keyEntity, err := ceRepo.LatestKey(keyIDs[0])
		require.NoError(t, err)
		require.Equal(t, key1Versions[2], keyEntity.Version)
	})

	t.Run("Create existing key", func(t *testing.T) {
		keyEntity, err := ceRepo.LatestKey(keyIDs[0])
		require.NoError(t, err)
		sameEntity, err := ceRepo.CreateKey(keyEntity)
		require.NoError(t, err)
		require.Equal(t, keyEntity, sameEntity) //ensure no new entity was created
	})

	t.Run("Get non-existing latest keys", func(t *testing.T) {
		_, err := ceRepo.LatestKey("Idontexist")
		require.Error(t, err)
		require.IsType(t, &EntityNotFoundError{}, err)
		require.True(t, IsNotFoundError(err))
	})

	t.Run("Get key by id and version", func(t *testing.T) {
		keyEntity, err := ceRepo.Key(keyIDs[0], key1Versions[1])
		require.NoError(t, err)
		require.Equal(t, key1Versions[1], keyEntity.Version)
	})

	t.Run("Get key by version", func(t *testing.T) {
		keyEntity, err := ceRepo.KeyByVersion(key1Versions[1])
		require.NoError(t, err)
		require.Equal(t, key1Versions[1], keyEntity.Version)
	})

	t.Run("Get non-existing key with keys", func(t *testing.T) {
		_, err := ceRepo.Key("Idontexist", -5)
		require.Error(t, err)
		require.IsType(t, &EntityNotFoundError{}, err)
		require.True(t, IsNotFoundError(err))
	})

	t.Run("Delete key(s)", func(t *testing.T) {
		for _, keyID := range keyIDs {
			keyEntity, err := ceRepo.LatestKey(keyID)
			require.NoError(t, err)
			err = ceRepo.DeleteKey(keyEntity.Key)
			require.NoError(t, err)
			keyEntities, err := ceRepo.KeyHistory(keyID)
			require.NoError(t, err)
			require.Equal(t, 0, len(keyEntities))
		}
	})
}

func TestConfigConfigRepositoryValues(t *testing.T) {
	var err error
	ceRepo := newKeyValueRepo(t)

	//create test key
	keyEntity, err := ceRepo.CreateKey(&KeyEntity{
		Key:      fmt.Sprintf("testKey%d", time.Now().UnixNano()),
		DataType: String,
		Username: "testUsername",
	})
	require.NoError(t, err)
	require.NotEmpty(t, keyEntity)

	bucketNames := []string{"test-bucket1", "test-bucket2"} //contains the bucket names used for the created test value entities

	value1Versions := []int64{} //contains the three versions of the first test value
	value2Versions := []int64{} //contains the three versions of the second test value

	t.Run("Validate entity and create test data", func(t *testing.T) {
		valueEntity := &ValueEntity{}
		_, err = ceRepo.CreateValue(valueEntity)
		require.True(t, db.IsInvalidEntityError(err))

		valueEntity.Key = keyEntity.Key
		_, err = ceRepo.CreateValue(valueEntity)
		require.True(t, db.IsInvalidEntityError(err))

		valueEntity.KeyVersion = keyEntity.Version
		_, err = ceRepo.CreateValue(valueEntity)
		require.True(t, db.IsInvalidEntityError(err))

		valueEntity.Username = "testUsername"
		_, err = ceRepo.CreateValue(valueEntity)
		require.True(t, db.IsInvalidEntityError(err))

		valueEntity.Bucket = bucketNames[0]
		_, err = ceRepo.CreateValue(valueEntity)
		require.True(t, db.IsInvalidEntityError(err))

		valueEntity.DataType = String
		_, err = ceRepo.CreateValue(valueEntity)
		require.True(t, db.IsInvalidEntityError(err))

		//create the first test value (added to bucket 'bucketNames[0]') in 3 versions
		for _, value := range []string{"entity1-value1", "entity1-value2", "entity1-value3"} {
			valueEntity.Value = value
			valueEntity, err := ceRepo.CreateValue(valueEntity)
			require.NoError(t, err)
			value1Versions = append(value1Versions, valueEntity.Version)
		}

		//create the second test value (added to bucket 'bucketNames[1]') in 3 versions
		value2Entity := &ValueEntity{
			Key:        keyEntity.Key,
			KeyVersion: keyEntity.Version,
			Bucket:     bucketNames[1],
			DataType:   String,
			Username:   "testUsername2",
		}
		for _, value := range []string{"entity2-value1", "entity2-value2", "entity2-value3"} {
			value2Entity.Value = value
			value2Entity, err = ceRepo.CreateValue(value2Entity)
			require.NoError(t, err)
			value2Versions = append(value2Versions, value2Entity.Version)
		}
		require.NoError(t, err)
	})

	t.Run("Get value history", func(t *testing.T) {
		valueEntities, err := ceRepo.ValueHistory(bucketNames[0], keyEntity.Key)
		require.NoError(t, err)
		require.Equal(t, 3, len(valueEntities))
		//ordered by version ASC:
		require.True(t, valueEntities[0].Version < valueEntities[1].Version && valueEntities[1].Version < valueEntities[2].Version)
	})

	t.Run("Get latest value", func(t *testing.T) {
		valueEntity, err := ceRepo.LatestValue(bucketNames[0], keyEntity.Key)
		require.NoError(t, err)
		require.Equal(t, value1Versions[2], valueEntity.Version)
	})

	t.Run("Create existing value", func(t *testing.T) {
		valueEntity, err := ceRepo.LatestValue(bucketNames[0], keyEntity.Key)
		require.NoError(t, err)
		sameEntity, err := ceRepo.CreateValue(valueEntity)
		require.NoError(t, err)
		require.Equal(t, valueEntity, sameEntity) //ensure no new entity was created
	})

	t.Run("Get non-existing latest value", func(t *testing.T) {
		_, err := ceRepo.LatestValue("Idontexist", "Idontexisttoo")
		require.Error(t, err)
		require.IsType(t, &EntityNotFoundError{}, err)
		require.True(t, IsNotFoundError(err))
	})

	t.Run("Get value with version", func(t *testing.T) {
		valueEntity, err := ceRepo.Value(bucketNames[0], keyEntity.Key, value1Versions[1])
		require.NoError(t, err)
		require.Equal(t, value1Versions[1], valueEntity.Version)
	})

	t.Run("Get non-existing value with version", func(t *testing.T) {
		_, err := ceRepo.Value("Idontexist", "Idontexisttoo", -1)
		require.Error(t, err)
		require.IsType(t, &EntityNotFoundError{}, err)
		require.True(t, IsNotFoundError(err))
	})

	t.Run("Get values by bucket", func(t *testing.T) {
		bucketEntities, err := ceRepo.Buckets()
		require.NoError(t, err)
		require.Equal(t, 2, len(bucketEntities))
		for _, bucketEntity := range bucketEntities {
			valueEntities, err := ceRepo.ValuesByBucket(bucketEntity.Bucket)
			require.NoError(t, err)
			require.Equal(t, 1, len(valueEntities))
			//for each bucket the latest value has to be returned
			if bucketEntity.Bucket == bucketNames[0] {
				require.Equal(t, value1Versions[2], valueEntities[0].Version)
			} else if bucketEntity.Bucket == bucketNames[1] {
				require.Equal(t, value2Versions[2], valueEntities[0].Version)
			} else {
				require.Fail(t, "Unexpected state: bucket name '%s' unknown", bucketEntity.Bucket)
			}
		}
	})

	t.Run("Get values by key", func(t *testing.T) {
		valueEntities, err := ceRepo.ValuesByKey(keyEntity)
		require.NoError(t, err)
		require.Equal(t, 2, len(valueEntities))

		//we expect the latest versions of both test values (as they are both in different buckets)
		expectedVersions := []int64{value1Versions[2], value2Versions[2]}
		for _, valueEntity := range valueEntities {
			require.Contains(t, expectedVersions, valueEntity.Version)
		}
	})

	t.Run("Get buckets", func(t *testing.T) {
		valueEntity := &ValueEntity{ //create second bucket
			Key:        keyEntity.Key,
			KeyVersion: keyEntity.Version,
			Username:   "xyz123",
			Bucket:     bucketNames[1],
			DataType:   String,
			Value:      "another value",
		}
		_, err := ceRepo.CreateValue(valueEntity)
		require.NoError(t, err)
		bucketEnitities, err := ceRepo.Buckets()
		require.NoError(t, err)

		//at least the buckets created during this test run have to exist:
		require.True(t, len(bucketEnitities) >= 2)

		//check that expected bucket were returned
		bucketNamesGot := []string{}
		for _, bucketEntity := range bucketEnitities {
			bucketNamesGot = append(bucketNamesGot, bucketEntity.Bucket)
		}
		for _, bucketNameExpected := range bucketNames {
			require.Contains(t, bucketNamesGot, bucketNameExpected)
		}

	})

	t.Run("Delete bucket(s)", func(t *testing.T) {
		for _, bucketName := range bucketNames {
			err := ceRepo.DeleteBucket(bucketName)
			require.NoError(t, err)
		}
		bucketEntities, err := ceRepo.Buckets()
		require.NoError(t, err)
		bucketNamesGot := []string{}
		for _, bucketEntity := range bucketEntities {
			bucketNamesGot = append(bucketNamesGot, bucketEntity.Bucket)
		}
		for _, bucketNameNotExpected := range bucketNames {
			require.NotContains(t, bucketNamesGot, bucketNameNotExpected)
		}
	})
}

func newKeyValueRepo(t *testing.T) *KeyValueRepository {
	connFact, err := newConnectionFactory()
	require.NoError(t, err)
	ceRepo, err := NewKeyValueRepository(connFact, true)
	require.NoError(t, err)
	return ceRepo
}
