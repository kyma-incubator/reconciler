package config

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/stretchr/testify/require"
)

func TestEntryRepository(t *testing.T) {
	ceRepo, err := NewConfigEntryRepository(&db.PostgresConnectionFactory{
		Host:     "localhost",
		User:     "kyma",
		Password: "kyma",
		Database: "kyma",
		Debug:    true,
	})
	require.NoError(t, err)
	err = ceRepo.CreateKey(&KeyEntity{
		Key:       "testKey",
		DataType:  String,
		Encrypted: true,
		User:      "abc",
	})
	require.Error(t, err, "Kyma model incomplete")
}
