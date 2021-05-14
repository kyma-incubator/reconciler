package config

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/stretchr/testify/require"
)

func TestCacheRepository(t *testing.T) {
	repo := newCacheRepo(t)

	t.Run("Creating cache entry", func(t *testing.T) {
		var err error
		cacheEntry := &CacheEntryEntity{
			Label: "cacheentry1",
		}
		cacheEntry, err = repo.Add(cacheEntry)
		require.True(t, db.IsIncompleteEntityError(err))

		cacheEntry.Cluster = "abc"
		cacheEntry, err = repo.Add(cacheEntry)
		require.True(t, db.IsIncompleteEntityError(err))

		cacheEntry.Buckets = "default,dev,abc"
		cacheEntry, err = repo.Add(cacheEntry)
		require.True(t, db.IsIncompleteEntityError(err))

		cacheEntry.Data = "The cached data goes here" //m5d: a3daa753769a78e732d763d143235d87
		cacheEntry, err = repo.Add(cacheEntry)
		require.NoError(t, err)
		require.Equal(t, "a3daa753769a78e732d763d143235d87", cacheEntry.checksum())
		require.True(t, cacheEntry.ID > 0)
	})

	//TODO: add further tests here
}

func newCacheRepo(t *testing.T) *CacheRepository {
	connFact, err := newConnectionFactory()
	require.NoError(t, err)
	ceRepo, err := NewCacheRepository(connFact, true)
	require.NoError(t, err)
	return ceRepo
}
