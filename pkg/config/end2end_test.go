package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	chart    = "test-chart"
	cluster  = "test-cluster"
	username = "e2etest"
)

func TestConfigurationManagement(t *testing.T) {
	cacheRepo := newCacheRepo(t)
	kvRepo := newKeyValueRepo(t)
	bucketMerger := BucketMerger{
		KeyValueRepository: kvRepo,
	}

	createTestData(t, kvRepo)

	//1. Check for cached chart
	cacheEntity, err := cacheRepo.Get(chart, cluster)
	require.Error(t, err)
	require.True(t, IsNotFoundError(err))
	require.Empty(t, cacheEntity)

	//2. TODO: lookup bucket-sequence which should be used for merging (implement ClusterConfiguration repository)

	//3. Get values which should be used for chart rendering (merge the bucket-sequence from step 2)
	//FIXME: bucket-merger should no longer know anything about a merge-sequence.. it should just merge the provided bucket-sequence
	values, err := bucketMerger.Merge(map[string]string{"landscape": "e2etest", "customer": "e2etest"})
	require.NoError(t, err)
	require.Equal(t, values.Len(), 2)

	//4. TODO: render chart
	renderedChart := fmt.Sprintf("I am the rendered charts using some of the provided values: %s", values)

	//5. Cache chart
	newCacheEntry, err := cacheRepo.Add(&CacheEntryEntity{
		Label:   chart,
		Cluster: cluster,
		Data:    renderedChart,
	}, values.ValuesList())
	require.NoError(t, err)
	require.NotEmpty(t, newCacheEntry)

	//Apply some sanity checks
	cacheDeps, err := cacheRepo.cache.Get().Exec()
	require.NoError(t, err)
	require.Len(t, cacheDeps, values.Len())

	err = cacheRepo.Invalidate(newCacheEntry.Label, newCacheEntry.Cluster)
	require.NoError(t, err)

	cacheDepsNew, err := cacheRepo.cache.Get().Exec()
	require.NoError(t, err)
	require.Len(t, cacheDepsNew, 0)

	//Cleanup
	buckets, err := kvRepo.Buckets()
	require.NoError(t, err)
	for _, bucket := range buckets {
		err := kvRepo.DeleteBucket(bucket.Bucket)
		require.NoError(t, err)
	}
}

func createTestData(t *testing.T, kvRepo *KeyValueRepository) {
	//creating test data
	key1, err := kvRepo.CreateKey(&KeyEntity{
		Key:       "testKey1",
		DataType:  String,
		Validator: "len(it) > 15",
		Username:  username,
	})
	require.NoError(t, err)
	key2, err := kvRepo.CreateKey(&KeyEntity{
		Key:       "testKey2",
		DataType:  Integer,
		Validator: "it > 100",
		Username:  username,
	})
	require.NoError(t, err)

	_, err = kvRepo.CreateValue(&ValueEntity{
		Key:        key1.Key,
		KeyVersion: key1.Version,
		Bucket:     "landscape-e2etest",
		DataType:   key1.DataType,
		Value:      "I am value1 in landscape-e2etest => WILL BE OVERWRITTEN",
		Username:   username,
	})
	require.NoError(t, err)
	_, err = kvRepo.CreateValue(&ValueEntity{
		Key:        key1.Key,
		KeyVersion: key1.Version,
		Bucket:     "customer-e2etest",
		DataType:   key1.DataType,
		Value:      "I am value1 in customer-e2etest => OUTDATED",
		Username:   username,
	})
	require.NoError(t, err)
	_, err = kvRepo.CreateValue(&ValueEntity{
		Key:        key1.Key,
		KeyVersion: key1.Version,
		Bucket:     "customer-e2etest",
		DataType:   key1.DataType,
		Value:      "I am value1 in customer-e2etest",
		Username:   username,
	})
	require.NoError(t, err)
	_, err = kvRepo.CreateValue(&ValueEntity{
		Key:        key2.Key,
		KeyVersion: key2.Version,
		Bucket:     "customer-e2etest",
		DataType:   key2.DataType,
		Value:      "999",
		Username:   username,
	})
	require.NoError(t, err)
}
