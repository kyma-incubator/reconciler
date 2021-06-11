package cmd

import (
	"github.com/kyma-incubator/reconciler/pkg/kv"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

type valueProcessor struct {
	repo   *kv.Repository
	values []*model.ValueEntity
	err    error
}

func newValueProcessor(repo *kv.Repository, key *model.KeyEntity) (*valueProcessor, error) {
	var err error
	valueProcessor := &valueProcessor{
		repo: repo,
	}
	valueProcessor.values, err = repo.ValuesByKey(key)
	return valueProcessor, err
}

func (v *valueProcessor) get() ([]*model.ValueEntity, error) {
	return v.values, v.err
}

func (v *valueProcessor) withHistory() *valueProcessor {
	valuesHistory := []*model.ValueEntity{}
	var valueHistory []*model.ValueEntity
	for _, value := range v.values {
		valueHistory, v.err = v.repo.ValueHistory(value.Bucket, value.Key)
		if v.err != nil {
			return v
		}
		valuesHistory = append(valuesHistory, valueHistory...)
	}
	v.values = valuesHistory
	return v
}
