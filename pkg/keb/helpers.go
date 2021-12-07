package keb

import "fmt"

func ToStatus(in string) (Status, error) {

	for _, status := range []Status{
		StatusDeleteError,
		StatusDeleteErrorRetryable,
		StatusDeletePending,
		StatusDeleted,
		StatusDeleting,
		StatusError,
		StatusReady,
		StatusReconcileDisabled,
		StatusReconcileErrorRetryable,
		StatusReconcilePending,
		StatusReconciling,
	} {
		if in == string(status) {
			return status, nil
		}
	}
	return Status(""), fmt.Errorf("Given string is not Status: %s", in)
}
