package repository

import (
	"bytes"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/pkg/errors"
)

func (r *Repository) NewNotFoundError(err error, entity db.DatabaseEntity,
	identifier map[string]interface{}) error {
	return &EntityNotFoundError{
		entity:     entity,
		identifier: identifier,
		err:        err,
	}
}

type EntityNotFoundError struct {
	entity     db.DatabaseEntity
	identifier map[string]interface{}
	err        error
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

func (e *EntityNotFoundError) Is(err error) bool {
	_, ok := err.(*EntityNotFoundError)
	return ok
}

func (e *EntityNotFoundError) Unwrap() error {
	return e.err
}

func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, &EntityNotFoundError{})
}
