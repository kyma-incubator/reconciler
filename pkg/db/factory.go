package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

func NewConnectionFactory(configFile string) (ConnectionFactory, error) {
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	dbToUse := viper.GetString("db.driver")
	switch dbToUse {
	case "postgres":
		return &PostgresConnectionFactory{
			Host:     viper.GetString("db.postgres.host"),
			Port:     viper.GetInt("db.postgres.port"),
			Database: viper.GetString("db.postgres.database"),
			User:     viper.GetString("db.postgres.user"),
			Password: viper.GetString("db.postgres.password"),
			SslMode:  viper.GetBool("db.postgres.sslMode"),
			Debug:    true,
		}, nil
	case "sqlite":
		dbFile := viper.GetString("db.sqlite.file")
		if viper.GetBool("db.sqlite.createFile") {
			dbFileDir := filepath.Dir(dbFile)
			if _, err := os.Stat(dbFileDir); os.IsNotExist(err) {
				if err := os.MkdirAll(dbFileDir, 0700); err != nil {
					return nil, err
				}
			}
		}
		return &SqliteConnectionFactory{
			File:  dbFile,
			Debug: true,
		}, nil
	default:
		panic(fmt.Sprintf("DB type '%s' not supported", dbToUse))
	}
}
