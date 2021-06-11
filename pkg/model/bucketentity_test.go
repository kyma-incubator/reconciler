package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClusterConfiguration(t *testing.T) {

	t.Run("Validate bucket name", func(t *testing.T) {
		require.NoError(t, ValidateBucketName(DefaultBucket))
		require.NoError(t, ValidateBucketName("1-1"))
		require.NoError(t, ValidateBucketName("a-z"))
		require.NoError(t, ValidateBucketName("abc-abc"))
		require.NoError(t, ValidateBucketName("123-123"))
		require.NoError(t, ValidateBucketName("1ab-ab2"))
		require.Error(t, ValidateBucketName("A-z"))     //no capital letters
		require.Error(t, ValidateBucketName("abc"))     //- is mandatory for non-default bucket
		require.Error(t, ValidateBucketName("abc abc")) //whitespaces not allowed
		require.Error(t, ValidateBucketName("abc-abc "))
		require.Error(t, ValidateBucketName(" abc-abc"))
		require.Error(t, ValidateBucketName("-abc")) // word before and after underscore required
		require.Error(t, ValidateBucketName("abc-"))
		require.Error(t, ValidateBucketName("abc- abc"))
		require.Error(t, ValidateBucketName("abc -abc"))
		require.Error(t, ValidateBucketName("abc - abc"))
		require.Error(t, ValidateBucketName("abc-abc-"))
		require.Error(t, ValidateBucketName("-abc-abc"))
	})
}
