package config

import (
	"fmt"
	"regexp"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
)

const DefaultBucket = "default"

var DefaultMergeSequence = []string{DefaultBucket, "landscape", "customer", "cluster", "feature"}

var bucketPattern = regexp.MustCompile(fmt.Sprintf(`^(%s|([a-z0-9]+(-[a-z0-9]+)+))$`, DefaultBucket))

type BucketMerger struct {
	KeyValueRepository *KeyValueRepository
}

func (bm *BucketMerger) Merge(buckets map[string]string) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	for _, bucket := range DefaultMergeSequence { //TODO: add support for cluster specific merge sequences: new DB entity required
		if bucket != DefaultBucket {
			subBucket, ok := buckets[bucket]
			if !ok {
				continue
			}
			bucket = fmt.Sprintf("%s-%s", bucket, subBucket)
		}
		values, err := bm.KeyValueRepository.ValuesByBucket(bucket)
		if err != nil {
			return result, err
		}
		valueMap, err := bm.valueMap(values)
		if err != nil {
			return result, err
		}
		if err := mergo.MergeWithOverwrite(&result, valueMap); err != nil {
			return result, errors.Wrap(err, fmt.Sprintf("Failed to merge value entries of bucket '%s'", bucket))
		}
	}
	return result, nil
}

func (bm *BucketMerger) valueMap(values []*ValueEntity) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(values))
	for _, value := range values {
		typedValue, err := value.Get()
		if err != nil {
			return result, errors.Wrap(err,
				fmt.Sprintf("Potential data inconsistency detected: failed to get typed value of value-entity %s", value))
		}
		result[value.Key] = typedValue
	}
	return result, nil
}

func ValidateBucketName(bucket string) error {
	if bucketPattern.MatchString(bucket) {
		return nil
	}
	return fmt.Errorf("Bucket name '%s' is invalid: bucket name has to match the pattern '%s'", bucket, bucketPattern)
}
