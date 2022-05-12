package keb

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/mitchellh/mapstructure"
)

type ModelFactory struct {
	version int64
}

func NewModelFactory(contractV int64) *ModelFactory {
	return &ModelFactory{contractV}
}

func (mf *ModelFactory) load(model interface{}, data io.Reader) (interface{}, error) {
	decoder := json.NewDecoder(data)
	switch mf.version { //add here further case statement if multiple contract versions have to be supported
	case 1:
		err := decoder.Decode(&model)
		return model, err
	default:
		return nil, fmt.Errorf("contract version '%d' not supported", mf.version)
	}
}

func (mf *ModelFactory) Status(data io.Reader) (*StatusUpdate, error) {
	model, err := mf.load(&StatusUpdate{}, data)
	if err != nil {
		return nil, err
	}
	return model.(*StatusUpdate), err
}

func (mf *ModelFactory) Metadata(data io.Reader) (*Metadata, error) {
	model, err := mf.load(&Metadata{}, data)
	if err != nil {
		return nil, err
	}
	return model.(*Metadata), err
}

func (mf *ModelFactory) Runtime(data io.Reader) (*RuntimeInput, error) {
	model, err := mf.load(&RuntimeInput{}, data)
	if err != nil {
		return nil, err
	}
	return model.(*RuntimeInput), err
}

func (mf *ModelFactory) Cluster(data io.Reader) (*Cluster, error) {
	model, err := mf.load(&Cluster{}, data)
	if err != nil {
		return nil, err
	}
	return model.(*Cluster), err
}

func (mf *ModelFactory) Components(data io.Reader) ([]*Component, error) {
	untypedModels, err := mf.load([]interface{}{}, data)
	if err != nil {
		return nil, err
	}
	var result []*Component
	if untypedModels == nil {
		return result, nil
	}
	for _, untypedModel := range untypedModels.([]interface{}) {
		typedModel := &Component{}
		err := mapstructure.Decode(untypedModel, typedModel)
		if err != nil {
			return result, err
		}
		result = append(result, typedModel)
	}
	return result, err
}

func (mf *ModelFactory) Administrators(data io.Reader) ([]string, error) {
	untypedModels, err := mf.load([]interface{}{}, data)
	if err != nil {
		return nil, err
	}
	result := []string{}
	for _, untypedModel := range untypedModels.([]interface{}) {
		if err != nil {
			return result, err
		}
		result = append(result, untypedModel.(string))
	}
	return result, err
}
