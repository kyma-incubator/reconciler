package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

func NewConnectionFactory(configFile, configSection string) (ConnectionFactory, error) {
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	dbToUse := viper.GetString(fmt.Sprintf("%s.db.driver", configSection))
	switch dbToUse {
	case "postgres":
		return &PostgresConnectionFactory{
			Host:     viper.GetString(fmt.Sprintf("%s.db.postgres.host", configSection)),
			Port:     viper.GetInt(fmt.Sprintf("%s.db.postgres.port", configSection)),
			Database: viper.GetString(fmt.Sprintf("%s.db.postgres.database", configSection)),
			User:     viper.GetString(fmt.Sprintf("%s.db.postgres.user", configSection)),
			Password: viper.GetString(fmt.Sprintf("%s.db.postgres.password", configSection)),
			SslMode:  viper.GetBool(fmt.Sprintf("%s.db.postgres.sslMode", configSection)),
			Debug:    true,
		}, nil
	case "sqlite":
		dbFile := viper.GetString(fmt.Sprintf("%s.db.sqlite.file", configSection))
		if viper.GetBool(fmt.Sprintf("%s.db.sqlite.createFile", configSection)) {
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
