package config

import (
	"fmt"
	"io/ioutil"
	"path"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/spf13/viper"
)

func newTestConnectionFactory() (db.ConnectionFactory, error) {
	viper.SetConfigFile(path.Join("test", "reconciler-test.yaml"))
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	dbToUse := viper.GetString("configManagement.db.driver")
	switch dbToUse {
	case "postgres":
		return &db.PostgresConnectionFactory{
			Host:     viper.GetString("configManagement.db.postgres.host"),
			Port:     viper.GetInt("configManagement.db.postgres.port"),
			Database: viper.GetString("configManagement.db.postgres.database"),
			User:     viper.GetString("configManagement.db.postgres.user"),
			Password: viper.GetString("configManagement.db.postgres.password"),
			SslMode:  viper.GetBool("configManagement.db.postgres.sslMode"),
			Debug:    true,
		}, nil
	case "sqlite":
		conFac := &db.SqliteConnectionFactory{
			File:  viper.GetString("configManagement.db.sqlite.file"),
			Debug: true,
		}
		//get connection
		conn, err := conFac.NewConnection()
		if err != nil {
			panic(err)
		}

		//read DDL (test-table structure)
		ddl, err := ioutil.ReadFile(path.Join("test", "configuration-management.sql"))
		if err != nil {
			panic(err)
		}

		//populate DB schema
		_, err = conn.Exec(string(ddl))
		if err != nil {
			panic(err)
		}
		return conFac, nil
	default:
		panic(fmt.Sprintf("DB type '%s' not supported", dbToUse))
	}
}
