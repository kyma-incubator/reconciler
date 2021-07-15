package db

import (
	"fmt"
	"os"
	"path/filepath"

	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/spf13/viper"
)

func NewConnectionFactory(configFile string, debug bool) (ConnectionFactory, error) {
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	dbToUse := viper.GetString("db.driver")
	switch dbToUse {
	case "postgres":
		connFact := &PostgresConnectionFactory{
			Host:     viper.GetString("db.postgres.host"),
			Port:     viper.GetInt("db.postgres.port"),
			Database: viper.GetString("db.postgres.database"),
			User:     viper.GetString("db.postgres.user"),
			Password: viper.GetString("db.postgres.password"),
			SslMode:  viper.GetBool("db.postgres.sslMode"),
			Debug:    debug,
		}
		return connFact, connFact.Init()
	case "sqlite":
		dbFile := viper.GetString("db.sqlite.file")
		//ensure directory structure of db-file exists
		dbFileDir := filepath.Dir(dbFile)
		if !file.DirExists(dbFile) {
			if err := os.MkdirAll(dbFileDir, 0700); err != nil {
				return nil, err
			}
		}
		//create the factory
		connFact := &SqliteConnectionFactory{
			File:  dbFile,
			Debug: debug,
			Reset: viper.GetBool("db.sqlite.resetDatabase"),
		}
		if viper.GetBool("db.sqlite.deploySchema") {
			connFact.SchemaFile = filepath.Join(filepath.Dir(viper.ConfigFileUsed()), "db", "sqlite", "reconciler.sql")
		}
		return connFact, connFact.Init()
	default:
		panic(fmt.Sprintf("DB type '%s' not supported", dbToUse))
	}
}
