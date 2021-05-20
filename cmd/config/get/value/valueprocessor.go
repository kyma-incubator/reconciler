package cmd

import (
	"github.com/kyma-incubator/reconciler/pkg/config"
)

type valueProcessor struct {
	repo   *config.KeyValueRepository
	values []*config.ValueEntity
	err    error
}

func newValueProcessor(repo *config.KeyValueRepository, key *config.KeyEntity) (*valueProcessor, error) {
	var err error
	valueProcessor := &valueProcessor{
		repo: repo,
	}
	valueProcessor.values, err = repo.ValuesByKey(key)
	return valueProcessor, err
}

func (v *valueProcessor) get() ([]*config.ValueEntity, error) {
	return v.values, v.err
}

func (v *valueProcessor) withHistory() *valueProcessor {
	valuesHistory := []*config.ValueEntity{}
	var valueHistory []*config.ValueEntity
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
