package config

import (
	"bytes"
	"database/sql"
	"fmt"
	"reflect"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"go.uber.org/zap"
)

type Repository struct {
	conn   db.Connection
	logger *zap.Logger
}

func NewRepository(dbFac db.ConnectionFactory, debug bool) (*Repository, error) {
	logger, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	conn, err := dbFac.NewConnection()
	return &Repository{
		conn:   conn,
		logger: logger,
	}, err
}

func (cer *Repository) handleNotFoundError(err error, entity db.DatabaseEntity,
	identifier map[string]interface{}) error {
	if err == sql.ErrNoRows {
		return &EntityNotFoundError{
			entity:     entity,
			identifier: identifier,
		}
	}
	return err
}

type EntityNotFoundError struct {
	entity     db.DatabaseEntity
	identifier map[string]interface{}
}

func (e *EntityNotFoundError) Error() string {
	var idents bytes.Buffer
	if e.identifier != nil {
		for k, v := range e.identifier {
			if idents.Len() > 0 {
				idents.WriteRune(',')
			}
			idents.WriteString(fmt.Sprintf("%s=%v", k, v))
		}
	}
	return fmt.Sprintf("Entity of type '%T' with identifier '%v' not found", e.entity, idents.String())
}

func IsNotFoundError(err error) bool {
	return reflect.TypeOf(err) == reflect.TypeOf(&EntityNotFoundError{})
}
