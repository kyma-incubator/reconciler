package test

import (
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/stretchr/testify/require"
	"log"
	"testing"
)

func CleanUpTables(t *testing.T) {
	dbConn := NewTestConnection(t)
	log.Println("after tests")
	queryPrefix := "TRUNCATE TABLE "
	if dbConn.Type() == db.SQLite {
		queryPrefix = "DELETE FROM "
	}
	tableNames := [...]string{
		"config_values",
		"config_keys",
		"config_cachedeps",
		"config_cache",
		"scheduler_operations",
		"scheduler_reconciliations",
		"inventory_clusters",
		"inventory_cluster_configs",
		"inventory_cluster_config_statuses",
	}
	for _, tableName := range tableNames {
		_, err := dbConn.DB().Exec(queryPrefix + tableName)
		require.NoError(t, err)
	}

}

func NewTestConnectionFactory(t *testing.T) db.ConnectionFactory {
	configFile, err := GetConfigFile()
	require.NoError(t, err)

	connFac, err := db.NewConnectionFactory(configFile, false, true)
	require.NoError(t, err)

	require.NoError(t, connFac.Init(false))
	return connFac
}

func NewTestConnection(t *testing.T) db.Connection {
	connFac := NewTestConnectionFactory(t)
	conn, err := connFac.NewConnection()
	require.NoError(t, err)
	return conn
}
