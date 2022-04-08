package db

import "fmt"

type InvalidEntityError struct {
	errorMsg string
}

func (e *InvalidEntityError) Error() string {
	return e.errorMsg
}

func NewInvalidEntityError(err string, args ...interface{}) *InvalidEntityError {
	return &InvalidEntityError{
		errorMsg: fmt.Sprintf(err, args...),
	}
}

func IsInvalidEntityError(err error) bool {
	_, ok := err.(*InvalidEntityError)
	return ok
}

type NoRowsAffectedError struct {
	errorMsg string
}

func (e *NoRowsAffectedError) Error() string {
	return e.errorMsg
}

func NewNoRowsAffectedError(err string, args ...interface{}) *InvalidEntityError {
	return &InvalidEntityError{
		errorMsg: fmt.Sprintf(err, args...),
	}
}

func IsNoRowsAffectedError(err error) bool {
	_, ok := err.(*InvalidEntityError)
	return ok
}
