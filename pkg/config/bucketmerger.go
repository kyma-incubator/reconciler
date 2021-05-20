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

func (bm *BucketMerger) Merge(buckets map[string]string) (*BucketMergeResult, error) {
	result := map[string]*ValueEntity{}
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
			return nil, err
		}
		valueMap := bm.valueMap(values)
		if err := mergo.MergeWithOverwrite(&result, valueMap); err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("Failed to merge value entries of bucket '%s'", bucket))
		}
	}
	return &BucketMergeResult{result: result}, nil
}

func (bm *BucketMerger) valueMap(values []*ValueEntity) map[string]*ValueEntity {
	result := make(map[string]*ValueEntity, len(values))
	for _, value := range values {
		result[value.Key] = value
	}
	return result
}

func ValidateBucketName(bucket string) error {
	if bucketPattern.MatchString(bucket) {
		return nil
	}
	return fmt.Errorf("Bucket name '%s' is invalid: bucket name has to match the pattern '%s'", bucket, bucketPattern)
}

type BucketMergeResult struct {
	result map[string]*ValueEntity
}

func (mr *BucketMergeResult) Values() map[string]*ValueEntity {
	return mr.result
}

func (mr *BucketMergeResult) Value(value string) (*ValueEntity, error) {
	valueEntity, ok := mr.result[value]
	if !ok {
		return nil, fmt.Errorf("Merge result does not contain the value '%s'", value)
	}
	return valueEntity, nil
}

func (mr *BucketMergeResult) Get(value string) (interface{}, error) {
	valueEntity, ok := mr.result[value]
	if !ok {
		return nil, fmt.Errorf("Merge result does not contain the value '%s'", value)
	}
	return valueEntity.Get()
}

func (mr *BucketMergeResult) GetAll() (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(mr.result))
	for key, value := range mr.result {
		typedValue, err := value.Get()
		if err != nil {
			return result, errors.Wrap(err,
				fmt.Sprintf("Potential data inconsistency detected: failed to get typed value of value-entity %s", value))
		}
		result[key] = typedValue
	}
	return result, nil
}
