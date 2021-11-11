package utils

import (
	"fmt"
	"strconv"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
)

func ReadSecretKey(secret *v1.Secret, secretKey string) (string, error) {
	if secret.Data == nil {
		return "", fmt.Errorf("failed to read %s from nil secret data", secretKey)
	}
	secretValue, ok := secret.Data[secretKey]
	if !ok {
		return "", fmt.Errorf("%s is not found in secret", secretKey)
	}
	return string(secretValue), nil
}

func SetOverrideFromSecret(logger *zap.SugaredLogger, secret *v1.Secret, configuration map[string]interface{}, secretKey string, overridePath string) {
	secretValue, err := ReadSecretKey(secret, secretKey)
	if err != nil {
		logger.Errorf("Error while fetching %s from secret... Override for path [%s] will be generated : [%s]", secretKey, overridePath, err.Error())
		return
	}
	if secretValue != "" {
		if value, err := strconv.ParseBool(secretValue); err == nil {
			configuration[overridePath] = value
		} else {
			configuration[overridePath] = secretValue
		}
	}
}
