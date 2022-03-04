package db

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"sync"

	//add Postgres driver:
	_ "github.com/lib/pq"
	gormPg "gorm.io/driver/postgres"
	"gorm.io/gorm"

	//add migrator source:
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

var gormConfig = &gorm.Config{}

type sharedLockableDb struct {
	sync.Mutex
	instance *gorm.DB
}

var sharedDb = sharedLockableDb{
	sync.Mutex{},
	nil,
}

type postgresOrmConnection struct {
	*postgresConnection
	orm *gorm.DB
}

type OrmConnection interface {
	Connection
	DBOrErr() (*sql.DB, error)
}

func (poc *postgresOrmConnection) DB() *sql.DB {
	db, err := poc.DBOrErr()
	if err != nil {
		poc.logger.Warnf("error while fetching DB %v", err)
		return nil
	}
	return db
}

func (poc *postgresOrmConnection) DBOrErr() (*sql.DB, error) {
	db, err := poc.orm.DB()
	if err != nil {
		return nil, err
	}
	return db, nil
}

func (poc *postgresOrmConnection) Ping() error {
	poc.logger.Debugf("Postgres Ping(ORM)")
	return poc.DB().Ping()
}

func (poc *postgresOrmConnection) QueryRow(query string, args ...interface{}) (DataRow, error) {
	poc.logger.Debugf("Postgres QueryRow(ORM): %s | %v", query, args)
	if err := poc.validator.Validate(query); err != nil {
		return nil, err
	}
	db, dbErr := poc.DBOrErr()
	if dbErr != nil {
		return nil, dbErr
	}
	return db.QueryRow(query, args...), nil
}

func (poc *postgresOrmConnection) Query(query string, args ...interface{}) (DataRows, error) {
	poc.logger.Debugf("Postgres Query(ORM): %s | %v", query, args)
	if err := poc.validator.Validate(query); err != nil {
		return nil, err
	}
	db, dbErr := poc.DBOrErr()
	if dbErr != nil {
		return nil, dbErr
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		poc.logger.Errorf("Postgres Query(ORM) error: %s", err)
	}
	return rows, err
}

func (poc *postgresOrmConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	poc.logger.Debugf("Postgres Exec(ORM): %s | %v", query, args)
	if err := poc.validator.Validate(query); err != nil {
		return nil, err
	}
	db, dbErr := poc.DBOrErr()
	if dbErr != nil {
		return nil, dbErr
	}
	result, err := db.Exec(query, args...)
	if err != nil {
		poc.logger.Errorf("Postgres Exec(ORM) error: %s", err)
	}
	return result, err
}

func (poc *postgresOrmConnection) Begin() (*TxConnection, error) {
	poc.logger.Debug("Postgres Begin(ORM)")
	db, dbErr := poc.DBOrErr()
	if dbErr != nil {
		return nil, dbErr
	}
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	return NewTxConnection(tx, poc, poc.logger), nil
}

func (poc *postgresOrmConnection) Close() error {
	poc.logger.Debug("Postgres Close(ORM)")
	return poc.db.Close()
}

func (poc *postgresOrmConnection) Type() Type {
	return Postgres
}

func (pcf *postgresConnectionFactory) ormConfiguration() *gorm.Config {
	return gormConfig
}

func (pcf *postgresConnectionFactory) dslConnString() string {
	sslMode := "disable"
	if pcf.sslMode {
		sslMode = "require"
	}

	return fmt.Sprintf(
		"host=%s user=%s dbname=%s port=%s password=%s sslMode=%s",
		pcf.host,
		pcf.user,
		pcf.database,
		pcf.port,
		pcf.password,
		sslMode,
	)
}

func (pcf *postgresConnectionFactory) NewORMConnection(debug bool, encryptionKey string) (*postgresOrmConnection, error) {
	logger := log.NewLogger(debug)

	var (
		sqlDb     *sql.DB
		encryptor *Encryptor
	)

	sharedDb.Lock()
	if sharedDb.instance == nil {
		defer sharedDb.Unlock()
		if newDb, newDbErr := gorm.Open(gormPg.Open(pcf.dslConnString()), pcf.ormConfiguration()); newDbErr == nil {
			sharedDb.instance = newDb
		} else {
			return nil, newDbErr
		}
	} else {
		sharedDb.Unlock()
	}

	var err error

	if err == nil {
		sqlDb, err = sharedDb.instance.DB()
	}

	if err == nil {
		err = sqlDb.Ping()
	}

	if err == nil {
		encryptor, err = NewEncryptor(encryptionKey)
	}

	if err != nil {
		return nil, fmt.Errorf("error while establishing DB Connection %v", err)
	}

	return &postgresOrmConnection{
		postgresConnection: &postgresConnection{
			id:        uuid.NewString(),
			encryptor: encryptor,
			validator: NewValidator(false, logger),
			logger:    logger,
		},
		orm: sharedDb.instance,
	}, nil
}
