package db

import (
	"fmt"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
)

func NewConnectionFactory(configFile string, debug bool) (ConnectionFactory, error) {
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	dbToUse := viper.GetString("db.driver")
	switch dbToUse {
	case "postgres":
		connFact := createPostgresConnectionFactory(debug)
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
			File:          dbFile,
			Debug:         debug,
			Reset:         viper.GetBool("db.sqlite.resetDatabase"),
			EncryptionKey: viper.GetString("db.encryption.key"),
		}
		if viper.GetBool("db.sqlite.deploySchema") {
			connFact.SchemaFile = filepath.Join(filepath.Dir(viper.ConfigFileUsed()), "db", "sqlite", "reconciler.sql")
		}
		return connFact, connFact.Init()
	default:
		panic(fmt.Sprintf("DB type '%s' not supported", dbToUse))
	}
}

func createPostgresConnectionFactory(debug bool) *PostgresConnectionFactory {
	host := viper.GetString("db.postgres.host")
	port := viper.GetInt("db.postgres.port")
	database := viper.GetString("db.postgres.database")
	user := viper.GetString("db.postgres.user")
	password := viper.GetString("db.postgres.password")
	sslMode := viper.GetBool("db.postgres.sslMode")
	encryptionKey := viper.GetString("db.encryption.key")

	if viper.IsSet("DATABASE_HOST") {
		host = viper.GetString("DATABASE_HOST")
	}
	if viper.IsSet("DATABASE_PORT") {
		port = viper.GetInt("DATABASE_PORT")
	}
	if viper.IsSet("DATABASE_NAME") {
		database = viper.GetString("DATABASE_NAME")
	}
	if viper.IsSet("DATABASE_USER") {
		user = viper.GetString("DATABASE_USER")
	}
	if viper.IsSet("DATABASE_PASSWORD") {
		password = viper.GetString("DATABASE_PASSWORD")
	}
	if viper.IsSet("DATABASE_SSL_MODE") {
		sslMode = viper.GetBool("DATABASE_SSL_MODE")
	}
	if viper.IsSet("DATABASE_ENCRYPTION_KEY") {
		encryptionKey = viper.GetString("DATABASE_ENCRYPTION_KEY")
	}

	return &PostgresConnectionFactory{
		Host:          host,
		Port:          port,
		Database:      database,
		User:          user,
		Password:      password,
		SslMode:       sslMode,
		EncryptionKey: encryptionKey,
		Debug:         debug,
	}
}
