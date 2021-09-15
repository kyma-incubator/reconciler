package keb

import "github.com/pkg/errors"

func ToStatus(in string) (Status, error) {
	value := Status(in)

	if value == "" {
		return "", errors.Errorf("Given string is not Status: %s", in)
	}
	return value, nil
}
