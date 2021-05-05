package cmd

import (
	"sort"

	"github.com/kyma-incubator/reconciler/pkg/config"
)

type keyProcessor struct {
	repo *config.ConfigEntryRepository
	keys []*config.KeyEntity
	err  error
}

func newKeyProcessor(repo *config.ConfigEntryRepository) (*keyProcessor, error) {
	var err error
	keyProcessor := &keyProcessor{
		repo: repo,
	}
	keyProcessor.keys, err = repo.Keys()
	return keyProcessor, err
}

func (k *keyProcessor) get() ([]*config.KeyEntity, error) {
	return k.keys, k.err
}

func (k *keyProcessor) withHistory() *keyProcessor {
	keysHistory := []*config.KeyEntity{}
	var keyHistory []*config.KeyEntity
	for _, key := range k.keys {
		keyHistory, k.err = k.repo.KeyHistory(key.Key)
		if k.err != nil {
			return k
		}
		keysHistory = append(keysHistory, keyHistory...)
	}
	k.keys = keysHistory
	return k
}

func (k *keyProcessor) filter(keyFilter []string) *keyProcessor {
	if len(keyFilter) == 0 {
		return k
	}

	//to improve speed, use map from bucket-name to bucket-entity
	keyByName := make(map[string]*config.KeyEntity, len(keyFilter))
	for _, key := range k.keys {
		keyByName[key.Key] = key
	}

	//filter keys
	filteredKeys := []*config.KeyEntity{}
	sort.Strings(keyFilter) //ensure the filtered keys are added to result in alphabetical order
	for _, filter := range keyFilter {
		if key, ok := keyByName[filter]; ok {
			filteredKeys = append(filteredKeys, key)
		}
	}

	k.keys = filteredKeys
	return k
}
