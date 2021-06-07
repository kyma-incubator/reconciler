package db

import (
	"fmt"

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
		return &SqliteConnectionFactory{
			File:  viper.GetString(fmt.Sprintf("%s.db.sqlite.file", configSection)),
			Debug: true,
		}, nil
	default:
		panic(fmt.Sprintf("DB type '%s' not supported", dbToUse))
	}
}
