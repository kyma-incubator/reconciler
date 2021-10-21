package keb

import "github.com/pkg/errors"

func ToStatus(in string) (Status, error) {

	for _, status := range []Status{
		StatusReconciling,
		StatusReconcilePending,
		StatusReconcileFailed,
		StatusReady,
		StatusError,
		StatusReconcileFailed,
	} {
		if in == string(status) {
			return status, nil
		}
	}
	return Status(""), errors.Errorf("Given string is not Status: %s", in)
}
