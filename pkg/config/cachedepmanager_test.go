package config

import (
	"testing"

	"github.com/fatih/structs"
	"github.com/stretchr/testify/require"
)

var expectedDeps = []*CacheDependencyEntity{
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
		Bucket:  "bucket3",
		Label:   "testCacheEntry2",
		Cluster: "testCluster2",
		CacheID: 2,
	},
}

func TestCacheDependencyManager(t *testing.T) {
	cacheDepMgr := newCacheDepManager(t)

	t.Run("Create dependencies", func(t *testing.T) {
		withTestData(t, cacheDepMgr, func(t *testing.T) {
			deps, err := cacheDepMgr.Get().Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, deps, expectedDeps)
		})
	})

	t.Run("Invalidate dependencies by non-existing key", func(t *testing.T) {
		withTestData(t, cacheDepMgr, func(t *testing.T) {
			err := cacheDepMgr.Invalidate().WithKey("key1234").Exec(true)
			require.NoError(t, err)

			deps, err := cacheDepMgr.Get().Exec()
			require.NoError(t, err)

			//key 'key1234' will cause invalidation of no cache entries
			compareCacheDepEntities(t, deps, []*CacheDependencyEntity{
				expectedDeps[0], expectedDeps[1], expectedDeps[2], expectedDeps[3], expectedDeps[4],
			})
		})
	})

	t.Run("Invalidate dependencies by key", func(t *testing.T) {
		withTestData(t, cacheDepMgr, func(t *testing.T) {
			err := cacheDepMgr.Invalidate().WithKey("key4").Exec(true)
			require.NoError(t, err)

			deps, err := cacheDepMgr.Get().Exec()
			require.NoError(t, err)

			//key 'key4' will cause invalidation of all deps referring to testCacheEntry2
			compareCacheDepEntities(t, deps, []*CacheDependencyEntity{
				expectedDeps[0], expectedDeps[1], expectedDeps[2],
			})
		})
	})

	t.Run("Invalidate dependencies by bucket", func(t *testing.T) {
		withTestData(t, cacheDepMgr, func(t *testing.T) {
			err := cacheDepMgr.Invalidate().WithBucket("bucket1").Exec(true)
			require.NoError(t, err)

			deps, err := cacheDepMgr.Get().Exec()
			require.NoError(t, err)

			//bucket 'bucket1' will cause invalidation of all deps
			compareCacheDepEntities(t, deps, []*CacheDependencyEntity{})
		})
	})

	t.Run("Invalidate dependencies by label", func(t *testing.T) {
		withTestData(t, cacheDepMgr, func(t *testing.T) {
			err := cacheDepMgr.Invalidate().WithLabel("testCacheEntry1").Exec(true)
			require.NoError(t, err)

			deps, err := cacheDepMgr.Get().Exec()
			require.NoError(t, err)

			//label 'testCacheEntry1' will cause invalidation of all deps referring to testCacheEntry1
			compareCacheDepEntities(t, deps, []*CacheDependencyEntity{
				expectedDeps[3], expectedDeps[4],
			})
		})
	})

	t.Run("Invalidate dependencies by cluster", func(t *testing.T) {
		withTestData(t, cacheDepMgr, func(t *testing.T) {
			err := cacheDepMgr.Invalidate().WithCluster("testCluster2").Exec(true)
			require.NoError(t, err)

			deps, err := cacheDepMgr.Get().Exec()
			require.NoError(t, err)

			//cluster 'testCluster2' will cause invalidation of all deps referring to testCacheEntry2
			compareCacheDepEntities(t, deps, []*CacheDependencyEntity{
				expectedDeps[0], expectedDeps[1], expectedDeps[2],
			})
		})
	})

	t.Run("Invalidate dependencies by cache-id", func(t *testing.T) {
		withTestData(t, cacheDepMgr, func(t *testing.T) {
			err := cacheDepMgr.Invalidate().WithCacheID(1).Exec(true)
			require.NoError(t, err)

			deps, err := cacheDepMgr.Get().Exec()
			require.NoError(t, err)

			//cache-id '1' will cause invalidation of all deps referring to testCacheEntry1
			compareCacheDepEntities(t, deps, []*CacheDependencyEntity{
				expectedDeps[3], expectedDeps[4],
			})
		})
	})

	t.Run("Get dependencies", func(t *testing.T) {
		withTestData(t, cacheDepMgr, func(t *testing.T) {
			depsByCacheID, err := cacheDepMgr.Get().WithCacheID(2).Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByCacheID, []*CacheDependencyEntity{
				expectedDeps[3], expectedDeps[4],
			})

			depsByBucket, err := cacheDepMgr.Get().WithBucket("bucket2").Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByBucket, []*CacheDependencyEntity{
				expectedDeps[2],
			})

			depsByCluster, err := cacheDepMgr.Get().WithCluster("testCluster1").Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByCluster, []*CacheDependencyEntity{
				expectedDeps[0], expectedDeps[1], expectedDeps[2],
			})

			depsByKey, err := cacheDepMgr.Get().WithKey("key1").Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByKey, []*CacheDependencyEntity{
				expectedDeps[0], expectedDeps[3],
			})

			depsByLabel, err := cacheDepMgr.Get().WithLabel("testCacheEntry2").Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByLabel, []*CacheDependencyEntity{
				expectedDeps[3], expectedDeps[4],
			})

			depsByKeyAndLabel, err := cacheDepMgr.Get().WithKey("key3").WithLabel("testCacheEntry1").Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByKeyAndLabel, []*CacheDependencyEntity{
				expectedDeps[2],
			})
		})
	})
}

func withTestData(t *testing.T, cacheDepMgr *cacheDependencyManager, testFct func(*testing.T)) {
	createTestData(t, cacheDepMgr)
	testFct(t)
	deleteTestData(t, cacheDepMgr)
}

func createTestData(t *testing.T, cacheDepMgr *cacheDependencyManager) {
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
				Bucket: "bucket3",
			},
		}).Exec(true)
	require.NoError(t, err)
}

func deleteTestData(t *testing.T, cacheDepMgr *cacheDependencyManager) {
	err := cacheDepMgr.Invalidate().Exec(true)
	require.NoError(t, err)
}

func newCacheDepManager(t *testing.T) *cacheDependencyManager {
	connFact, err := newTestConnectionFactory()
	if err != nil {
		require.NoError(t, err)
	}
	conn, err := connFact.NewConnection()
	if err != nil {
		require.NoError(t, err)
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
