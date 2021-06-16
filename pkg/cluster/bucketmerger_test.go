package cluster

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestBucketMerger(t *testing.T) {

	t.Run("Merge buckets", func(t *testing.T) {
		bm := &bucketMerger{}

		err := bm.Add("bucket1", []*model.ValueEntity{
			{
				Key:      "key1",
				DataType: model.String,
				Value:    "abc",
			},
		})
		require.NoError(t, err)

		err = bm.Add("bucket2", []*model.ValueEntity{
			{
				Key:      "key1",
				DataType: model.String,
				Value:    "xyz",
			},
			{
				Key:      "key2",
				DataType: model.Integer,
				Value:    "123",
			},
		})
		require.NoError(t, err)

		err = bm.Add("bucket3", []*model.ValueEntity{
			{
				Key:      "key3",
				DataType: model.Boolean,
				Value:    "true",
			},
		})
		require.NoError(t, err)

		//check result
		values, err := bm.GetAll()
		require.NoError(t, err)

		require.NoError(t, err)
		require.Equal(t, map[string]interface{}{"key1": "xyz", "key2": int64(123), "key3": true}, values)
	})

}
