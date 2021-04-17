package config

import "github.com/kyma-incubator/reconciler/pkg/db"

type ConfigEntryRepository struct {
	Factor db.ConnectionFactory
}
