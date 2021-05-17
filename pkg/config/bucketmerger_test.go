package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBucketMerger(t *testing.T) {
	t.Run("Validate bucket name", func(t *testing.T) {
		require.NoError(t, ValidateBucketName(DefaultBucket))
		require.NoError(t, ValidateBucketName("1_1"))
		require.NoError(t, ValidateBucketName("a_z"))
		require.NoError(t, ValidateBucketName("abc_abc"))
		require.NoError(t, ValidateBucketName("123_123"))
		require.NoError(t, ValidateBucketName("1ab_ab2"))
		require.Error(t, ValidateBucketName("A_z"))     //no capital letters
		require.Error(t, ValidateBucketName("abc"))     //_ is mandatory for non-default bucket
		require.Error(t, ValidateBucketName("abc abc")) //whitespaces not allowed
		require.Error(t, ValidateBucketName("abc_abc "))
		require.Error(t, ValidateBucketName(" abc_abc"))
		require.Error(t, ValidateBucketName("_abc")) // word before and after underscore required
		require.Error(t, ValidateBucketName("abc_"))
		require.Error(t, ValidateBucketName("abc_ abc"))
		require.Error(t, ValidateBucketName("abc _abc"))
		require.Error(t, ValidateBucketName("abc _ abc"))
		require.Error(t, ValidateBucketName("abc_abc_"))
		require.Error(t, ValidateBucketName("_abc_abc"))
	})
}
