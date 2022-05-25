package db

import (
	"fmt"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"time"
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
			return nil, errors.Wrap(err, "error creating sqliteConnectionFactory")
		}
		return connFact, connFact.Init(migrate)

	default:
		return nil, fmt.Errorf("DB type '%s' not supported", dbToUse)
	}
}

func MigrateDatabase(configFile string, debug bool) error {
	_, err := NewConnectionFactory(configFile, true, debug)
	return err
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

type postgresEnvironment struct {
	host            string
	port            int
	database        string
	user            string
	password        string
	sslMode         bool
	migrationsDir   string
	maxOpenConns    int
	maxIdleConns    int
	connMaxIdleTime time.Duration
	connMaxLifetime time.Duration
}

func getPostgresEnvironment() postgresEnvironment {
	host := viper.GetString("db.postgres.Host")
	port := viper.GetInt("db.postgres.Port")
	database := viper.GetString("db.postgres.Database")
	user := viper.GetString("db.postgres.User")
	password := viper.GetString("db.postgres.Password")
	sslMode := viper.GetBool("db.postgres.sslMode")
	migrationsDir := viper.GetString("db.postgres.migrationsDir")
	maxOpenConns := viper.GetInt("db.postgres.maxOpenConns")
	maxIdleConns := viper.GetInt("db.postgres.maxIdleConns")
	connMaxLifetime := viper.GetDuration("db.postgres.connMaxLifetime")
	connMaxIdleTime := viper.GetDuration("db.postgres.connMaxIdleTime")

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
	if viper.IsSet("DATABASE_MAX_OPEN_CONNS") {
		maxOpenConns = viper.GetInt("DATABASE_MAX_OPEN_CONNS")
	}
	if viper.IsSet("DATABASE_MAX_IDLE_CONNS") {
		maxIdleConns = viper.GetInt("DATABASE_MAX_IDLE_CONNS")
	}
	if viper.IsSet("DATABASE_CONN_MAX_LIFETIME") {
		connMaxLifetime = viper.GetDuration("DATABASE_CONN_MAX_LIFETIME")
	}
	if viper.IsSet("DATABASE_CONN_MAX_IDLE_TIME") {
		connMaxIdleTime = viper.GetDuration("DATABASE_CONN_MAX_IDLE_TIME")
	}

	return postgresEnvironment{
		host:            host,
		port:            port,
		database:        database,
		user:            user,
		password:        password,
		sslMode:         sslMode,
		migrationsDir:   migrationsDir,
		maxOpenConns:    maxOpenConns,
		maxIdleConns:    maxIdleConns,
		connMaxIdleTime: connMaxIdleTime,
		connMaxLifetime: connMaxLifetime,
	}
}

func readEncryptionKey() (string, error) {
	encKeyFile := viper.GetString("db.encryption.keyFile")
	if encKeyFile != "" {
		if !filepath.IsAbs(encKeyFile) {
			//define absolute path relative to Config-file directory
			encKeyFile = filepath.Join(filepath.Dir(viper.ConfigFileUsed()), encKeyFile)
		}
	}

	//overwrite encKeyFile if env-var if defined
	if viper.IsSet("DATABASE_ENCRYPTION_KEYFILE") {
		encKeyFile = viper.GetString("DATABASE_ENCRYPTION_KEYFILE")
	}

	return readKeyFile(encKeyFile)
}

func createPostgresConnectionFactory(encKey string, debug bool, blockQueries, logQueries bool) *postgresConnectionFactory {

	env := getPostgresEnvironment()

	return &postgresConnectionFactory{
		host:            env.host,
		port:            env.port,
		database:        env.database,
		user:            env.user,
		password:        env.password,
		sslMode:         env.sslMode,
		encryptionKey:   encKey,
		migrationsDir:   env.migrationsDir,
		blockQueries:    blockQueries,
		logQueries:      logQueries,
		debug:           debug,
		maxOpenConns:    env.maxOpenConns,
		maxIdleConns:    env.maxIdleConns,
		connMaxIdleTime: env.connMaxIdleTime,
		connMaxLifetime: env.connMaxLifetime,
	}
}
