package cmd

import (
	"sort"

	"github.com/kyma-incubator/reconciler/pkg/config"
)

type keysProcessor struct {
	repo *config.ConfigEntryRepository
	keys []*config.KeyEntity
	err  error
}

func newKeysProcessor(repo *config.ConfigEntryRepository) (*keysProcessor, error) {
	var err error
	keyEntityProcessor := &keysProcessor{
		repo: repo,
	}
	keyEntityProcessor.keys, err = repo.Keys()
	return keyEntityProcessor, err
}

func (kl *keysProcessor) get() ([]*config.KeyEntity, error) {
	return kl.keys, kl.err
}

func (kl *keysProcessor) withHistory() *keysProcessor {
	keysHistory := []*config.KeyEntity{}
	var keyHistory []*config.KeyEntity
	for _, key := range kl.keys {
		keyHistory, kl.err = kl.repo.KeyHistory(key.Key)
		if kl.err != nil {
			return kl
		}
		keysHistory = append(keysHistory, keyHistory...)
	}
	kl.keys = keysHistory
	return kl
}

func (kl *keysProcessor) filter(keyFilter []string) *keysProcessor {
	if len(keyFilter) == 0 {
		return kl
	}

	//to improve speed, use map from bucket-name to bucket-entity
	keyByName := make(map[string]*config.KeyEntity, len(keyFilter))
	for _, key := range kl.keys {
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

	kl.keys = filteredKeys
	return kl
}
