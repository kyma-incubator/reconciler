package keb

import "fmt"

func ToStatus(in string) (Status, error) {

	for _, status := range []Status{
		StatusDeleteError,
		StatusDeletePending,
		StatusDeleted,
		StatusDeleting,
		StatusError,
		StatusReady,
		StatusReconcileDisabled,
		StatusReconcilePending,
		StatusReconciling,
	} {
		if in == string(status) {
			return status, nil
		}
	}
	return Status(""), fmt.Errorf("Given string is not Status: %s", in)
}
