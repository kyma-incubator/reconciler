package keb

import "github.com/pkg/errors"

func ToStatus(in string) (Status, error) {

	for _, status := range []Status{
		StatusReconcilePending,
		StatusReconciling,
		StatusReady,
		StatusError,
		StatusReconcileDisabled,
		StatusDeletePending,
		StatusDeleting,
		StatusDeleted,
		StatusDeleteError,
	} {
		if in == string(status) {
			return status, nil
		}
	}
	return Status(""), errors.Errorf("Given string is not Status: %s", in)
}
