package db

import (
	"fmt"
	"os"
	"path/filepath"

	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/spf13/viper"
)

func NewConnectionFactory(configFile string, migrate bool, debug bool) (ConnectionFactory, error) {
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	encKey, err := readEncryptionKey()
	if err != nil {
		return nil, err
	}

	dbToUse := viper.GetString("db.driver")
	blockQueries := viper.GetBool("db.blockQueries")
	logQueries := viper.GetBool("db.logQueries")

	switch dbToUse {
	case "postgres":
		connFact := createPostgresConnectionFactory(encKey, debug, blockQueries, logQueries)
		return connFact, connFact.Init(migrate)

	case "sqlite":
		connFact, err := createSqliteConnectionFactory(encKey, debug, blockQueries, logQueries)
		if err != nil {
			return nil, err
		}
		return connFact, connFact.Init(migrate)

	default:
		return nil, fmt.Errorf("DB type '%s' not supported", dbToUse)
	}
}

func readEncryptionKey() (string, error) {
	encKeyFile := viper.GetString("db.encryption.keyFile")
	if encKeyFile != "" {
		if !filepath.IsAbs(encKeyFile) {
			//define absolute path relative to config-file directory
			encKeyFile = filepath.Join(filepath.Dir(viper.ConfigFileUsed()), encKeyFile)
		}
	}

	//overwrite encKeyFile if env-var if defined
	if viper.IsSet("DATABASE_ENCRYPTION_KEYFILE") {
		encKeyFile = viper.GetString("DATABASE_ENCRYPTION_KEYFILE")
	}

	return readKeyFile(encKeyFile)
}

func createSqliteConnectionFactory(encKey string, debug bool, blockQueries, logQueries bool) (*sqliteConnectionFactory, error) {
	dbFile := viper.GetString("db.sqlite.file")
	//ensure directory structure of db-file exists
	dbFileDir := filepath.Dir(dbFile)
	if !file.DirExists(dbFile) {
		if err := os.MkdirAll(dbFileDir, 0700); err != nil {
			return nil, err
		}
	}
	connFact := &sqliteConnectionFactory{
		file:          dbFile,
		debug:         debug,
		reset:         viper.GetBool("db.sqlite.resetDatabase"),
		encryptionKey: encKey,
		blockQueries:  blockQueries,
		logQueries:    logQueries,
	}
	if viper.GetBool("db.sqlite.deploySchema") {
		connFact.schemaFile = filepath.Join(filepath.Dir(viper.ConfigFileUsed()), "db", "sqlite", "reconciler.sql")
	}
	return connFact, nil
}

func createPostgresConnectionFactory(encKey string, _ bool, blockQueries, logQueries bool) *postgresConnectionFactory {
	host := viper.GetString("db.postgres.host")
	port := viper.GetInt("db.postgres.port")
	database := viper.GetString("db.postgres.database")
	user := viper.GetString("db.postgres.user")
	password := viper.GetString("db.postgres.password")
	sslMode := viper.GetBool("db.postgres.sslMode")
	migrationsDir := viper.GetString("db.postgres.migrationsDir")

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
	if viper.IsSet("DATABASE_MIGRATIONS_DIR") {
		migrationsDir = viper.GetString("DATABASE_MIGRATIONS_DIR")
	}

	return &postgresConnectionFactory{
		host:          host,
		port:          port,
		database:      database,
		user:          user,
		password:      password,
		sslMode:       sslMode,
		encryptionKey: encKey,
		migrationsDir: migrationsDir,
		blockQueries:  blockQueries,
		logQueries:    logQueries,
	}
}
