package repository

import (
	"testing"

	"github.com/fatih/structs"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var cacheDep *cacheDependencyManager

func TestCacheDependencyManager(t *testing.T) {
	cacheDep = newCacheDependencyManager(db.NewTestConnection(t), true)

	t.Run("Create dependencies", func(t *testing.T) {
		withTestData(t, func(t *testing.T, testEntries []*model.CacheEntryEntity, testDeps []*model.CacheDependencyEntity) {
			deps, err := cacheDep.Get().Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, deps, testDeps)
		})
	})

	t.Run("Invalidate dependencies by non-existing key", func(t *testing.T) {
		withTestData(t, func(t *testing.T, testEntries []*model.CacheEntryEntity, testDeps []*model.CacheDependencyEntity) {
			err := cacheDep.Invalidate().WithKey("key1234").Exec(true)
			require.NoError(t, err)

			deps, err := cacheDep.Get().Exec()
			require.NoError(t, err)

			//key 'key1234' will cause invalidation of no cache entries
			compareCacheDepEntities(t, deps, []*model.CacheDependencyEntity{
				testDeps[0], testDeps[1], testDeps[2], testDeps[3], testDeps[4],
			})
		})
	})

	t.Run("Invalidate dependencies by key", func(t *testing.T) {
		withTestData(t, func(t *testing.T, testEntries []*model.CacheEntryEntity, testDeps []*model.CacheDependencyEntity) {
			err := cacheDep.Invalidate().WithKey("key4").Exec(true)
			require.NoError(t, err)

			deps, err := cacheDep.Get().Exec()
			require.NoError(t, err)

			//key 'key4' will cause invalidation of all deps referring to testCacheEntry2
			compareCacheDepEntities(t, deps, []*model.CacheDependencyEntity{
				testDeps[0], testDeps[1], testDeps[2],
			})
		})
	})

	t.Run("Invalidate dependencies by bucket", func(t *testing.T) {
		withTestData(t, func(t *testing.T, testEntries []*model.CacheEntryEntity, testDeps []*model.CacheDependencyEntity) {
			err := cacheDep.Invalidate().WithBucket("bucket1").Exec(true)
			require.NoError(t, err)

			deps, err := cacheDep.Get().Exec()
			require.NoError(t, err)

			//bucket 'bucket1' will cause invalidation of all deps
			compareCacheDepEntities(t, deps, []*model.CacheDependencyEntity{})
		})
	})

	t.Run("Invalidate dependencies by label", func(t *testing.T) {
		withTestData(t, func(t *testing.T, testEntries []*model.CacheEntryEntity, testDeps []*model.CacheDependencyEntity) {
			err := cacheDep.Invalidate().WithLabel("testCacheEntry1").Exec(true)
			require.NoError(t, err)

			deps, err := cacheDep.Get().Exec()
			require.NoError(t, err)

			//label 'testCacheEntry1' will cause invalidation of all deps referring to testCacheEntry1
			compareCacheDepEntities(t, deps, []*model.CacheDependencyEntity{
				testDeps[3], testDeps[4],
			})
		})
	})

	t.Run("Invalidate dependencies by cluster", func(t *testing.T) {
		withTestData(t, func(t *testing.T, testEntries []*model.CacheEntryEntity, testDeps []*model.CacheDependencyEntity) {
			err := cacheDep.Invalidate().WithRuntimeID("testCluster2").Exec(true)
			require.NoError(t, err)

			deps, err := cacheDep.Get().Exec()
			require.NoError(t, err)

			//cluster 'testCluster2' will cause invalidation of all deps referring to testCacheEntry2
			compareCacheDepEntities(t, deps, []*model.CacheDependencyEntity{
				testDeps[0], testDeps[1], testDeps[2],
			})
		})
	})

	t.Run("Invalidate dependencies by cache-id", func(t *testing.T) {
		withTestData(t, func(t *testing.T, testEntries []*model.CacheEntryEntity, testDeps []*model.CacheDependencyEntity) {
			err := cacheDep.Invalidate().WithCacheID(testEntries[0].ID).Exec(true)
			require.NoError(t, err)

			deps, err := cacheDep.Get().Exec()
			require.NoError(t, err)

			//cache-id[0] will cause invalidation of all deps referring to testCacheEntry1
			compareCacheDepEntities(t, deps, []*model.CacheDependencyEntity{
				testDeps[3], testDeps[4],
			})
		})
	})

	t.Run("Get dependencies", func(t *testing.T) {
		withTestData(t, func(t *testing.T, testEntries []*model.CacheEntryEntity, testDeps []*model.CacheDependencyEntity) {
			depsByCacheID, err := cacheDep.Get().WithCacheID(testEntries[1].ID).Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByCacheID, []*model.CacheDependencyEntity{
				testDeps[3], testDeps[4],
			})

			depsByBucket, err := cacheDep.Get().WithBucket("bucket2").Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByBucket, []*model.CacheDependencyEntity{
				testDeps[2],
			})

			depsByCluster, err := cacheDep.Get().WithRuntimeID("testCluster1").Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByCluster, []*model.CacheDependencyEntity{
				testDeps[0], testDeps[1], testDeps[2],
			})

			depsByKey, err := cacheDep.Get().WithKey("key1").Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByKey, []*model.CacheDependencyEntity{
				testDeps[0], testDeps[3],
			})

			depsByLabel, err := cacheDep.Get().WithLabel("testCacheEntry2").Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByLabel, []*model.CacheDependencyEntity{
				testDeps[3], testDeps[4],
			})

			depsByKeyAndLabel, err := cacheDep.Get().WithKey("key3").WithLabel("testCacheEntry1").Exec()
			require.NoError(t, err)
			compareCacheDepEntities(t, depsByKeyAndLabel, []*model.CacheDependencyEntity{
				testDeps[2],
			})
		})
	})
}

func withTestData(t *testing.T, testFunc func(*testing.T, []*model.CacheEntryEntity, []*model.CacheDependencyEntity)) {
	entity1, deps1 := importCacheEntry(t,
		&model.CacheEntryEntity{
			Label:     "testCacheEntry1",
			RuntimeID: "testCluster1",
			Data:      "test cache data1",
		},
		[]*model.ValueEntity{
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
	entity2, deps2 := importCacheEntry(t,
		&model.CacheEntryEntity{
			Label:     "testCacheEntry2",
			RuntimeID: "testCluster2",
			Data:      "test cache data2",
		},
		[]*model.ValueEntity{
			{
				Key:    "key1",
				Bucket: "bucket1",
			},
			{
				Key:    "key4",
				Bucket: "bucket3",
			},
		})

	expectedDeps := []*model.CacheDependencyEntity{}
	expectedDeps = append(expectedDeps, deps1...)
	expectedDeps = append(expectedDeps, deps2...)

	testFunc(t, []*model.CacheEntryEntity{entity1, entity2}, expectedDeps)
	require.NoError(t, cacheDep.Invalidate().Exec(true))
}

func importCacheEntry(t *testing.T, cacheEntry *model.CacheEntryEntity, cacheDeps []*model.ValueEntity) (*model.CacheEntryEntity, []*model.CacheDependencyEntity) {
	testLogger := zap.NewExample().Sugar()
	defer func() {
		if err := testLogger.Sync(); err != nil {
			t.Logf("while flushing logs: %s", err)
		}
	}()

	q, err := db.NewQuery(cacheDep.conn, cacheEntry, testLogger)
	require.NoError(t, err)

	//create new cache entry entity
	err = q.Insert().Exec()
	require.NoError(t, err)

	//track dependencies for this entity
	err = cacheDep.Record(cacheEntry, cacheDeps).Exec(true)
	require.NoError(t, err)

	//return recorded dependencies
	deps, err := cacheDep.Get().WithCacheID(cacheEntry.ID).Exec()
	require.NoError(t, err)

	return cacheEntry, deps
}

func compareCacheDepEntities(t *testing.T, got, expected []*model.CacheDependencyEntity) {
	require.Len(t, got, len(expected))
	for idx := range expected {
		expectedValue := []interface{}{}
		gotValue := []interface{}{}
		for _, fieldName := range []string{"Key", "Bucket", "Label", "CacheID", "RuntimeID"} {
			field, ok := structs.New(expected[idx]).FieldOk(fieldName)
			require.True(t, ok, "field: %s not found in expected struct:%v", fieldName, expected[idx])
			expectedValue = append(expectedValue, field.Value())

			field, ok = structs.New(got[idx]).FieldOk(fieldName)
			require.True(t, ok, "field: %s not found in got struct: %v", fieldName, got[idx])
			gotValue = append(gotValue, field.Value())
		}
		require.ElementsMatch(t, expectedValue, gotValue)
	}
}
