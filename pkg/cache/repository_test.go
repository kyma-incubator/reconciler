package cache

import (
	"github.com/kyma-incubator/reconciler/pkg/test"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/repository"
	"github.com/stretchr/testify/require"
)

func TestRepository(t *testing.T) {

	//nolint:unused
	repo := newCacheRepo(t)

	var cacheDeps []*model.ValueEntity = []*model.ValueEntity{
		{
			Key:    "depKey1",
			Bucket: "depBucket1",
		},
		{
			Key:    "depKey2",
			Bucket: "depBucket2",
		},
	}
	var cacheEntries []*model.CacheEntryEntity

	t.Run("Creating cache entries", func(t *testing.T) {
		var err error
		//test incomplete entry
		cacheEntry1 := &model.CacheEntryEntity{
			Label: "cacheentry1",
		}
		cacheEntry1, err = repo.Add(cacheEntry1, cacheDeps)
		require.True(t, db.IsInvalidEntityError(err))

		cacheEntry1.RuntimeID = "abc"
		cacheEntry1, err = repo.Add(cacheEntry1, cacheDeps)
		require.True(t, db.IsInvalidEntityError(err))

		cacheEntry1, err = repo.Add(cacheEntry1, cacheDeps)
		require.True(t, db.IsInvalidEntityError(err))

		//create entry1 (has cache dependencies)
		cacheEntry1.Data = "The cached data goes here" //m5d: a3daa753769a78e732d763d143235d87
		cacheEntry1, err = repo.Add(cacheEntry1, cacheDeps)
		require.NoError(t, err)
		require.Equal(t, "a3daa753769a78e732d763d143235d87", cacheEntry1.NewChecksum())
		require.True(t, cacheEntry1.ID > 0)
		cacheEntries = append(cacheEntries, cacheEntry1)

		//create entry2 (has cache dependencies)
		cacheEntry2 := &model.CacheEntryEntity{
			Label:     "cacheentry2",
			RuntimeID: "xyz",
			Data:      "The second cached data goes here", //md5: 3bb77817db259eed817165ef8d891e61
		}
		cacheEntry2, err = repo.Add(cacheEntry2, cacheDeps)
		require.NoError(t, err)
		require.Equal(t, "3bb77817db259eed817165ef8d891e61", cacheEntry2.NewChecksum())
		require.True(t, cacheEntries[0].ID < cacheEntry2.ID) //ID is an incremental counter
		cacheEntries = append(cacheEntries, cacheEntry2)

		//create entry3 (has NO cache dependencies)
		cacheEntry3 := &model.CacheEntryEntity{
			Label:     "cacheentry3",
			RuntimeID: "foo",
			Data:      "The third cached data goes here", //md5: dbdb486dafb60e21872b71ea14a0659c
		}
		cacheEntry3, err = repo.Add(cacheEntry3, nil)
		require.NoError(t, err)
		require.Equal(t, "dbdb486dafb60e21872b71ea14a0659c", cacheEntry3.NewChecksum())
		require.True(t, cacheEntries[1].ID < cacheEntry3.ID) //ID is an incremental counter
		cacheEntries = append(cacheEntries, cacheEntry3)
	})

	t.Run("Get cache entries", func(t *testing.T) {
		entries, err := repo.All()
		require.NoError(t, err)
		require.ElementsMatch(t, cacheEntries, entries)
	})

	t.Run("Creating cache entry twice", func(t *testing.T) {
		cacheEntry, err := repo.Add(&model.CacheEntryEntity{
			Label:     "cacheentry1",
			RuntimeID: "abc",
			Data:      "The cached data goes here",
		}, cacheDeps)
		require.NoError(t, err)
		require.Equal(t, "a3daa753769a78e732d763d143235d87", cacheEntry.NewChecksum())
		require.Equal(t, cacheEntries[0].ID, cacheEntry.ID)
	})

	t.Run("Get non existing cache entry", func(t *testing.T) {
		_, err := repo.Get("foo", "bar")
		require.Error(t, err)
		require.True(t, repository.IsNotFoundError(err))
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
		cacheEntry, err := repo.Add(&model.CacheEntryEntity{
			Label:     "cacheentry1",
			RuntimeID: "abc",
			Data:      "This is the updated cache entry", //md5: 38776bd2eb877254ff1350e1f088b1fd
		}, cacheDeps)
		require.NoError(t, err)
		require.Equal(t, "38776bd2eb877254ff1350e1f088b1fd", cacheEntry.NewChecksum())
		require.True(t, cacheEntries[0].ID < cacheEntry.ID) //ID is an incremental counter
		cacheEntries[0] = cacheEntry                        //replace first cache entry with the new created entry (caused by the update)
	})

	t.Run("Invalidate cache entry by label and cluster (deleted by deps)", func(t *testing.T) {
		//delete first entry
		entry1, err := repo.Get(cacheEntries[0].Label, cacheEntries[0].RuntimeID) //ensure entry exists
		require.NotEmpty(t, entry1)
		require.NoError(t, err)
		err = repo.Invalidate(entry1.Label, entry1.RuntimeID) //invalidate it
		require.NoError(t, err)
		_, err = repo.Get(cacheEntries[0].Label, cacheEntries[0].RuntimeID) //ensure entry1 no longer exists
		require.True(t, repository.IsNotFoundError(err))
	})

	t.Run("Invalidate cache entry by id (deleted by deps)", func(t *testing.T) {
		//delete second entry
		entry2, err := repo.GetByID(cacheEntries[1].ID) //ensure entry exists
		require.NotEmpty(t, entry2)
		require.NoError(t, err)
		err = repo.InvalidateByID(entry2.ID) //invalidate it
		require.NoError(t, err)
		_, err = repo.GetByID(entry2.ID)
		require.True(t, repository.IsNotFoundError(err)) //ensure entry2 no longer exist
	})

	t.Run("Invalidate cache entry by id (deleted without deps)", func(t *testing.T) {
		//delete second entry
		entry2, err := repo.GetByID(cacheEntries[2].ID) //ensure entry exists
		require.NotEmpty(t, entry2)
		require.NoError(t, err)
		err = repo.InvalidateByID(entry2.ID) //invalidate it
		require.NoError(t, err)
		_, err = repo.GetByID(entry2.ID)
		require.True(t, repository.IsNotFoundError(err)) //ensure entry2 no longer exist
	})
}

//nolint:unused
func newCacheRepo(t *testing.T) *Repository {
	ceRepo, err := NewRepository(test.NewTestConnection(t), true)
	require.NoError(t, err)
	return ceRepo
}
