package config

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/stretchr/testify/require"
)

func TestCacheRepository(t *testing.T) {
	repo := newCacheRepo(t)

	var cacheDeps []*ValueEntity = []*ValueEntity{
		{
			Key:        "depKey1",
			KeyVersion: 1,
			Bucket:     "depBucket1",
		},
		{
			Key:        "depKey2",
			KeyVersion: 2,
			Bucket:     "depBucket2",
		},
	}
	var cacheEntries []*CacheEntryEntity

	t.Run("Creating cache entries", func(t *testing.T) {
		var err error
		//test incomplete entry
		cacheEntry1 := &CacheEntryEntity{
			Label: "cacheentry1",
		}
		cacheEntry1, err = repo.Add(cacheEntry1, cacheDeps)
		require.True(t, db.IsInvalidEntityError(err))

		cacheEntry1.Cluster = "abc"
		cacheEntry1, err = repo.Add(cacheEntry1, cacheDeps)
		require.True(t, db.IsInvalidEntityError(err))

		cacheEntry1, err = repo.Add(cacheEntry1, cacheDeps)
		require.True(t, db.IsInvalidEntityError(err))

		//create entry1
		cacheEntry1.Data = "The cached data goes here" //m5d: a3daa753769a78e732d763d143235d87
		cacheEntry1, err = repo.Add(cacheEntry1, cacheDeps)
		require.NoError(t, err)
		require.Equal(t, "a3daa753769a78e732d763d143235d87", cacheEntry1.checksum())
		require.True(t, cacheEntry1.ID > 0)
		cacheEntries = append(cacheEntries, cacheEntry1)

		//create entry2
		cacheEntry2 := &CacheEntryEntity{
			Label:   "cacheentry2",
			Cluster: "xyz",
			Data:    "The second cached data goes here", //md5: 3bb77817db259eed817165ef8d891e61
		}
		cacheEntry2, err = repo.Add(cacheEntry2, cacheDeps)
		require.NoError(t, err)
		require.Equal(t, "3bb77817db259eed817165ef8d891e61", cacheEntry2.checksum())
		require.True(t, cacheEntries[0].ID < cacheEntry2.ID) //ID is an incremental counter
		cacheEntries = append(cacheEntries, cacheEntry2)
	})

	t.Run("Get cache entries", func(t *testing.T) {
		entries, err := repo.All()
		require.NoError(t, err)
		require.ElementsMatch(t, cacheEntries, entries)
	})

	t.Run("Creating cache entry twice", func(t *testing.T) {
		cacheEntry, err := repo.Add(&CacheEntryEntity{
			Label:   "cacheentry1",
			Cluster: "abc",
			Data:    "The cached data goes here",
		}, nil)
		require.NoError(t, err)
		require.Equal(t, "a3daa753769a78e732d763d143235d87", cacheEntry.checksum())
		require.Equal(t, cacheEntries[0].ID, cacheEntry.ID)
	})

	t.Run("Get non existing cache entry", func(t *testing.T) {
		_, err := repo.Get("foo", "bar")
		require.Error(t, err)
		require.True(t, IsNotFoundError(err))
	})

	t.Run("Get cache entry", func(t *testing.T) {
		cacheEntry, err := repo.Get("cacheentry1", "abc")
		require.NoError(t, err)
		require.Equal(t, cacheEntries[0].ID, cacheEntry.ID)
	})

	t.Run("Get cache entry by ID", func(t *testing.T) {
		cacheEntry, err := repo.GetByID(cacheEntries[0].ID)
		require.NoError(t, err)
		require.Equal(t, cacheEntries[0].ID, cacheEntry.ID)
	})

	t.Run("Update cache entry", func(t *testing.T) {
		cacheEntry, err := repo.Add(&CacheEntryEntity{
			Label:   "cacheentry1",
			Cluster: "abc",
			Data:    "This is the updated cache entry", //md5: 38776bd2eb877254ff1350e1f088b1fd
		}, nil)
		require.NoError(t, err)
		require.Equal(t, "38776bd2eb877254ff1350e1f088b1fd", cacheEntry.checksum())
		require.True(t, cacheEntries[0].ID < cacheEntry.ID) //ID is an incremental counter
		cacheEntries[0] = cacheEntry                        //replace first cache entry with the new created entry (caused by the update)
	})

	t.Run("Invalidate cache entry by label and cluster", func(t *testing.T) {
		//delete first entry
		entry1, err := repo.Get(cacheEntries[0].Label, cacheEntries[0].Cluster) //ensure entry exists
		require.NotEmpty(t, entry1)
		require.NoError(t, err)
		err = repo.Invalidate(entry1.Label, entry1.Cluster) //invalidate it
		require.NoError(t, err)
		_, err = repo.Get(cacheEntries[0].Label, cacheEntries[0].Cluster) //ensure entry1 no longer exists
		require.True(t, IsNotFoundError(err))
	})

	t.Run("Invalidate cache entry by id", func(t *testing.T) {
		//delete second entry
		entry2, err := repo.GetByID(cacheEntries[1].ID) //ensure entry exists
		require.NotEmpty(t, entry2)
		require.NoError(t, err)
		err = repo.InvalidateByID(entry2.ID) //invalidate it
		require.NoError(t, err)
		_, err = repo.GetByID(entry2.ID)
		require.True(t, IsNotFoundError(err)) //ensure entry2 no longer exist
	})
}

func newCacheRepo(t *testing.T) *CacheRepository {
	connFact, err := newTestConnectionFactory()
	require.NoError(t, err)
	ceRepo, err := NewCacheRepository(connFact, true)
	require.NoError(t, err)
	return ceRepo
}
