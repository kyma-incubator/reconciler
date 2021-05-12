package config

import (
	"github.com/kyma-incubator/reconciler/pkg/db"
)

type CacheRepository struct {
	*Repository
}

func NewCacheRepository(dbFac db.ConnectionFactory, debug bool) (*CacheRepository, error) {
	repo, err := NewRepository(dbFac, debug)
	if err != nil {
		return nil, err
	}
	return &CacheRepository{repo}, nil
}
