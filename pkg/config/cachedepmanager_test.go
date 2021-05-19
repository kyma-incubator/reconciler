package config

import (
	"testing"

	"github.com/fatih/structs"
	"github.com/stretchr/testify/require"
)

func TestCacheDependencyManager(t *testing.T) {
	cacheDepMgr := newCacheDepManager(t)

	t.Run("Create dependencies", func(t *testing.T) {
		//record deps for first cache entity
		err := cacheDepMgr.Record(
			&CacheEntryEntity{
				ID:      1,
				Label:   "testCacheEntry1",
				Cluster: "testCluster1",
				Data:    "test cache data1",
			},
			[]*ValueEntity{
				{
					Key:    "key1",
					Bucket: "bucket1",
				},
				{
					Key:    "key2",
					Bucket: "bucket1",
				},
				{
					Key:    "key3",
					Bucket: "bucket2",
				},
			}).Exec(true)
		require.NoError(t, err)

		//record deps for second cache entity
		err = cacheDepMgr.Record(
			&CacheEntryEntity{
				ID:      2,
				Label:   "testCacheEntry2",
				Cluster: "testCluster2",
				Data:    "test cache data2",
			},
			[]*ValueEntity{
				{
					Key:    "key1",
					Bucket: "bucket1",
				},
				{
					Key:    "key4",
					Bucket: "bucket2",
				},
			}).Exec(true)
		require.NoError(t, err)
	})

	//get deps and compare result
	deps, err := cacheDepMgr.Get().Exec()
	require.NoError(t, err)

	compareCacheDepEntities(t, deps, []*CacheDependencyEntity{
		//deps of cache entity 1
		{
			Key:     "key1",
			Bucket:  "bucket1",
			Label:   "testCacheEntry1",
			Cluster: "testCluster1",
			CacheID: 1,
		},
		{
			Key:     "key2",
			Bucket:  "bucket1",
			Label:   "testCacheEntry1",
			Cluster: "testCluster1",
			CacheID: 1,
		},
		{
			Key:     "key3",
			Bucket:  "bucket2",
			Label:   "testCacheEntry1",
			Cluster: "testCluster1",
			CacheID: 1,
		},
		//deps of cache entity 2
		{
			Key:     "key1",
			Bucket:  "bucket1",
			Label:   "testCacheEntry2",
			Cluster: "testCluster2",
			CacheID: 2,
		},
		{
			Key:     "key4",
			Bucket:  "bucket2",
			Label:   "testCacheEntry2",
			Cluster: "testCluster2",
			CacheID: 2,
		},
	})

}

func newCacheDepManager(t *testing.T) *cacheDependencyManager {
	connFact, err := newTestConnectionFactory()
	if err != nil {
		panic(err)
	}
	conn, err := connFact.NewConnection()
	if err != nil {
		panic(err)
	}
	cacheDepMgr, err := newCacheDependencyManager(conn, true)
	require.NoError(t, err)
	return cacheDepMgr
}

func compareCacheDepEntities(t *testing.T, got, expected []*CacheDependencyEntity) {
	require.Len(t, got, len(expected))
	for idx := range expected {
		expectedValue := []interface{}{}
		gotValue := []interface{}{}
		for _, field := range []string{"Key", "Bucket", "Label", "Cluster", "CacheID"} {
			expectedValue = append(expectedValue, structs.New(expected[idx]).Field(field).Value())
			gotValue = append(gotValue, structs.New(got[idx]).Field(field).Value())
		}
		require.ElementsMatch(t, expectedValue, gotValue)
	}
}
