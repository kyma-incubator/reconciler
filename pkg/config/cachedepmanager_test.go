package config

import (
	"testing"

	"github.com/fatih/structs"
	"github.com/stretchr/testify/require"
)

func TestCacheDependencyManager(t *testing.T) {
	cacheRepo := newCacheRepo(t)
	cacheDepMgr := cacheRepo.cacheDep

	t.Run("Create dependencies", func(t *testing.T) {
		withTestData(t, cacheRepo, func(t *testing.T, testEntries []*CacheEntryEntity, testDeps []*CacheDependencyEntity) {
			deps, err := cacheDepMgr.Get().Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, deps, testDeps)
		})
	})

	t.Run("Invalidate dependencies by non-existing key", func(t *testing.T) {
		withTestData(t, cacheRepo, func(t *testing.T, testEntries []*CacheEntryEntity, testDeps []*CacheDependencyEntity) {
			err := cacheDepMgr.Invalidate().WithKey("key1234").Exec(true)
			require.NoError(t, err)

			deps, err := cacheDepMgr.Get().Exec()
			require.NoError(t, err)

			//key 'key1234' will cause invalidation of no cache entries
			compareCacheDepEntities(t, deps, []*CacheDependencyEntity{
				testDeps[0], testDeps[1], testDeps[2], testDeps[3], testDeps[4],
			})
		})
	})

	t.Run("Invalidate dependencies by key", func(t *testing.T) {
		withTestData(t, cacheRepo, func(t *testing.T, testEntries []*CacheEntryEntity, testDeps []*CacheDependencyEntity) {
			err := cacheDepMgr.Invalidate().WithKey("key4").Exec(true)
			require.NoError(t, err)

			deps, err := cacheDepMgr.Get().Exec()
			require.NoError(t, err)

			//key 'key4' will cause invalidation of all deps referring to testCacheEntry2
			compareCacheDepEntities(t, deps, []*CacheDependencyEntity{
				testDeps[0], testDeps[1], testDeps[2],
			})
		})
	})

	t.Run("Invalidate dependencies by bucket", func(t *testing.T) {
		withTestData(t, cacheRepo, func(t *testing.T, testEntries []*CacheEntryEntity, testDeps []*CacheDependencyEntity) {
			err := cacheDepMgr.Invalidate().WithBucket("bucket1").Exec(true)
			require.NoError(t, err)

			deps, err := cacheDepMgr.Get().Exec()
			require.NoError(t, err)

			//bucket 'bucket1' will cause invalidation of all deps
			compareCacheDepEntities(t, deps, []*CacheDependencyEntity{})
		})
	})

	t.Run("Invalidate dependencies by label", func(t *testing.T) {
		withTestData(t, cacheRepo, func(t *testing.T, testEntries []*CacheEntryEntity, testDeps []*CacheDependencyEntity) {
			err := cacheDepMgr.Invalidate().WithLabel("testCacheEntry1").Exec(true)
			require.NoError(t, err)

			deps, err := cacheDepMgr.Get().Exec()
			require.NoError(t, err)

			//label 'testCacheEntry1' will cause invalidation of all deps referring to testCacheEntry1
			compareCacheDepEntities(t, deps, []*CacheDependencyEntity{
				testDeps[3], testDeps[4],
			})
		})
	})

	t.Run("Invalidate dependencies by cluster", func(t *testing.T) {
		withTestData(t, cacheRepo, func(t *testing.T, testEntries []*CacheEntryEntity, testDeps []*CacheDependencyEntity) {
			err := cacheDepMgr.Invalidate().WithCluster("testCluster2").Exec(true)
			require.NoError(t, err)

			deps, err := cacheDepMgr.Get().Exec()
			require.NoError(t, err)

			//cluster 'testCluster2' will cause invalidation of all deps referring to testCacheEntry2
			compareCacheDepEntities(t, deps, []*CacheDependencyEntity{
				testDeps[0], testDeps[1], testDeps[2],
			})
		})
	})

	t.Run("Invalidate dependencies by cache-id", func(t *testing.T) {
		withTestData(t, cacheRepo, func(t *testing.T, testEntries []*CacheEntryEntity, testDeps []*CacheDependencyEntity) {
			err := cacheDepMgr.Invalidate().WithCacheID(testEntries[0].ID).Exec(true)
			require.NoError(t, err)

			deps, err := cacheDepMgr.Get().Exec()
			require.NoError(t, err)

			//cache-id[0] will cause invalidation of all deps referring to testCacheEntry1
			compareCacheDepEntities(t, deps, []*CacheDependencyEntity{
				testDeps[3], testDeps[4],
			})
		})
	})

	t.Run("Get dependencies", func(t *testing.T) {
		withTestData(t, cacheRepo, func(t *testing.T, testEntries []*CacheEntryEntity, testDeps []*CacheDependencyEntity) {
			depsByCacheID, err := cacheDepMgr.Get().WithCacheID(testEntries[1].ID).Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByCacheID, []*CacheDependencyEntity{
				testDeps[3], testDeps[4],
			})

			depsByBucket, err := cacheDepMgr.Get().WithBucket("bucket2").Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByBucket, []*CacheDependencyEntity{
				testDeps[2],
			})

			depsByCluster, err := cacheDepMgr.Get().WithCluster("testCluster1").Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByCluster, []*CacheDependencyEntity{
				testDeps[0], testDeps[1], testDeps[2],
			})

			depsByKey, err := cacheDepMgr.Get().WithKey("key1").Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByKey, []*CacheDependencyEntity{
				testDeps[0], testDeps[3],
			})

			depsByLabel, err := cacheDepMgr.Get().WithLabel("testCacheEntry2").Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByLabel, []*CacheDependencyEntity{
				testDeps[3], testDeps[4],
			})

			depsByKeyAndLabel, err := cacheDepMgr.Get().WithKey("key3").WithLabel("testCacheEntry1").Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByKeyAndLabel, []*CacheDependencyEntity{
				testDeps[2],
			})
		})
	})
}

func withTestData(t *testing.T, cacheRepo *CacheRepository, testFct func(*testing.T, []*CacheEntryEntity, []*CacheDependencyEntity)) {
	entity1, deps1 := importCacheEntry1(t, cacheRepo)
	entity2, deps2 := importCacheEntry2(t, cacheRepo)

	expectedDeps := []*CacheDependencyEntity{}
	expectedDeps = append(expectedDeps, deps1...)
	expectedDeps = append(expectedDeps, deps2...)

	testFct(t, []*CacheEntryEntity{entity1, entity2}, expectedDeps)
	require.NoError(t, cacheRepo.cacheDep.Invalidate().Exec(true))
}

func importCacheEntry1(t *testing.T, cacheRepo *CacheRepository) (*CacheEntryEntity, []*CacheDependencyEntity) {
	cacheEntry, err := cacheRepo.Add(
		&CacheEntryEntity{
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
		})
	require.NoError(t, err)

	return cacheEntry, []*CacheDependencyEntity{
		{
			Key:     "key1",
			Bucket:  "bucket1",
			Label:   "testCacheEntry1",
			Cluster: "testCluster1",
			CacheID: cacheEntry.ID,
		},
		{
			Key:     "key2",
			Bucket:  "bucket1",
			Label:   "testCacheEntry1",
			Cluster: "testCluster1",
			CacheID: cacheEntry.ID,
		},
		{
			Key:     "key3",
			Bucket:  "bucket2",
			Label:   "testCacheEntry1",
			Cluster: "testCluster1",
			CacheID: cacheEntry.ID,
		},
	}
}

func importCacheEntry2(t *testing.T, cacheRepo *CacheRepository) (*CacheEntryEntity, []*CacheDependencyEntity) {
	cacheEntry, err := cacheRepo.Add(
		&CacheEntryEntity{
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
		})
	require.NoError(t, err)

	return cacheEntry, []*CacheDependencyEntity{
		{
			Key:     "key1",
			Bucket:  "bucket1",
			Label:   "testCacheEntry2",
			Cluster: "testCluster2",
			CacheID: cacheEntry.ID,
		},
		{
			Key:     "key4",
			Bucket:  "bucket3",
			Label:   "testCacheEntry2",
			Cluster: "testCluster2",
			CacheID: cacheEntry.ID,
		},
	}
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
