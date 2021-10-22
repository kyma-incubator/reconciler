package connectivityproxy

import (
	"github.com/pkg/errors"
	"strings"
)

type Map map[string]interface{}

func (u Map) getSecretName() (string, error) {
	secretName := u.getValue(strings.Split("spec.secretName", ".")...)
	if secretName == nil {
		return "", errors.New("Secret not found")
	}
	return secretName.(string), nil
}

func (u Map) getValue(path ...string) interface{} {
	currentValue, ok := u[path[0]]
	if !ok {
		return nil
	}

	casted, ok := currentValue.(map[string]interface{})

	var m Map
	if ok {
		m = casted
	}

	if !ok && len(path) == 1 {
		return currentValue
	} else if !ok && len(path) > 1 {
		return nil
	}

	return m.getValue(path[1:]...)
}
