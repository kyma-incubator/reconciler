package config

import (
	"fmt"
	"io/ioutil"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestBucketMerger(t *testing.T) {
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

	t.Run("Merge buckets (all attributes provided)", func(t *testing.T) {
		bucketGroups := map[string]string{
			"landscape": "unittest",
			"customer":  "testcustomer",
			"cluster":   "testcluster",
			"feature":   "testfeature",
		}
		kvRepo := newKeyValueRepo(t)
		initRepo(t, kvRepo, bucketGroups)
		bm := &BucketMerger{
			KeyValueRepository: kvRepo,
		}
		result, err := bm.Merge(bucketGroups)
		require.NoError(t, err)
		require.Equal(t, loadYaml(t, "expected.yaml"), result)
	})

}

func initRepo(t *testing.T, kvRepo *KeyValueRepository, buckets map[string]string) {
	for _, bucket := range DefaultMergeSequence {
		if bucket != DefaultBucket {
			subBucket, ok := buckets[bucket]
			if !ok { //bucket not in buckets map - ignore it
				continue
			}
			bucket = fmt.Sprintf("%s-%s", bucket, subBucket)
		}
		kvMap := loadYaml(t, fmt.Sprintf("%s.yaml", bucket))
		importKVMap(t, kvRepo, bucket, kvMap)
	}
}

func loadYaml(t *testing.T, bucketFile string) map[string]interface{} {
	result := map[string]interface{}{}
	yamlData, err := ioutil.ReadFile(path.Join("test", "merger", bucketFile))
	require.NoError(t, err)
	err = yaml.Unmarshal(yamlData, result)
	require.NoError(t, err)
	return result
}

func importKVMap(t *testing.T, kvRepo *KeyValueRepository, bucket string, data map[string]interface{}) {
	for key, value := range data {
		keyEntity, err := kvRepo.CreateKey(&KeyEntity{
			Key:      key,
			DataType: String,
			Username: "unittest",
		})
		require.NoError(t, err)
		require.NotEmpty(t, keyEntity)
		valueEntity, err := kvRepo.CreateValue(&ValueEntity{
			Key:        keyEntity.Key,
			KeyVersion: keyEntity.Version,
			Bucket:     bucket,
			DataType:   String,
			Value:      fmt.Sprintf("%s", value),
			Username:   "unittest",
		})
		require.NoError(t, err)
		require.NotEmpty(t, valueEntity)
	}
}
