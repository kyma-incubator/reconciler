package cluster

import (
	"fmt"

	"github.com/imdario/mergo"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/pkg/errors"
)

type bucketMerger struct {
	result map[string]*model.ValueEntity
}

func (bm *bucketMerger) Add(bucket string, values []*model.ValueEntity) error {
	if err := mergo.MergeWithOverwrite(&bm.result, bm.keyValueMap(values)); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to merge value entries of bucket '%s'", bucket))
	}
	return nil
}

func (bm *bucketMerger) keyValueMap(values []*model.ValueEntity) map[string]*model.ValueEntity {
	result := make(map[string]*model.ValueEntity, len(values))
	for _, value := range values {
		result[value.Key] = value
	}
	return result
}

func (bm *bucketMerger) ValuesList() []*model.ValueEntity {
	result := []*model.ValueEntity{}
	for _, v := range bm.result {
		result = append(result, v)
	}
	return result
}

func (bm *bucketMerger) Values() map[string]*model.ValueEntity {
	return bm.result
}

func (bm *bucketMerger) Value(value string) (*model.ValueEntity, error) {
	valueEntity, ok := bm.result[value]
	if !ok {
		return nil, fmt.Errorf("Merge result does not contain the value '%s'", value)
	}
	return valueEntity, nil
}

func (bm *bucketMerger) Get(value string) (interface{}, error) {
	valueEntity, ok := bm.result[value]
	if !ok {
		return nil, fmt.Errorf("Merge result does not contain the value '%s'", value)
	}
	return valueEntity.Get()
}

func (bm *bucketMerger) GetAll() (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(bm.result))
	for key, value := range bm.result {
		typedValue, err := value.Get()
		if err != nil {
			return result, errors.Wrap(err,
				fmt.Sprintf("Potential data inconsistency detected: failed to get typed value of value-entity %s", value))
		}
		result[key] = typedValue
	}
	return result, nil
}

func (bm *bucketMerger) Len() int {
	return len(bm.result)
}
