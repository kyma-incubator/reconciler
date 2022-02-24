package provisioning

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/provisioning/gardener"
	"github.com/pkg/errors"
)

func toGardenerConfig(configuration map[string]interface{}) (gardener.GardenerConfig, error) {
	name, err := getStringKey(configuration, "name")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	projectName, err := getStringKey(configuration, "projectName")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	kubernetesVersion, err := getStringKey(configuration, "kubernetesVersion")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	volumeSizeGB, err := getIntPointerKey(configuration, "volumeSizeGB")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	diskType, err := getStringPointerKey(configuration, "diskType")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	machineType, err := getStringKey(configuration, "MachineType")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	machineImage, err := getStringPointerKey(configuration, "MachineImage")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	provider, err := getStringKey(configuration, "provider")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	purpose, err := getStringPointerKey(configuration, "purpose")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	licenceType, err := getStringPointerKey(configuration, "licenceType")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	seed, err := getStringKey(configuration, "seed")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	targetSecret, err := getStringKey(configuration, "targetSecret")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	region, err := getStringKey(configuration, "region")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	workerCidr, err := getStringKey(configuration, "workerCidr")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	autoScalerMin, err := getIntKey(configuration, "autoScalerMin")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	autoScalerMax, err := getIntKey(configuration, "autoScalerMax")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	maxSurge, err := getIntKey(configuration, "maxSurge")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	maxUnavailable, err := getIntKey(configuration, "maxUnavailable")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	enableKubernetesVersionAutoUpdate, err := getBoolKey(configuration, "enableKubernetesVersionAutoUpdate")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	enableMachineImageVersionAutoUpdate, err := getBoolKey(configuration, "enableMachineImageVersionAutoUpdate")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	allowPrivilegedContainers, err := getBoolKey(configuration, "allowPrivilegedContainers")
	if err != nil {
		return gardener.GardenerConfig{}, err
	}

	return gardener.GardenerConfig{
		Name:                                name,
		ProjectName:                         projectName,
		KubernetesVersion:                   kubernetesVersion,
		VolumeSizeGB:                        volumeSizeGB,
		DiskType:                            diskType,
		MachineType:                         machineType,
		MachineImage:                        machineImage,
		Provider:                            provider,
		Purpose:                             purpose,
		LicenceType:                         licenceType,
		Seed:                                seed,
		TargetSecret:                        targetSecret,
		Region:                              region,
		WorkerCidr:                          workerCidr,
		AutoScalerMin:                       autoScalerMin,
		AutoScalerMax:                       autoScalerMax,
		MaxSurge:                            maxSurge,
		MaxUnavailable:                      maxUnavailable,
		EnableKubernetesVersionAutoUpdate:   enableKubernetesVersionAutoUpdate,
		EnableMachineImageVersionAutoUpdate: enableMachineImageVersionAutoUpdate,
		AllowPrivilegedContainers:           allowPrivilegedContainers,
	}, nil
}

func getStringKey(configuration map[string]interface{}, keyName string) (string, error) {
	val, ok := configuration[keyName]
	if !ok {
		return "", errors.New(fmt.Sprintf("key %s not found", keyName))
	}

	valString, ok := val.(string)
	if !ok {
		return "", errors.New(fmt.Sprintf("string value type expected for key %s", keyName))
	}

	return valString, nil
}

func getIntKey(configuration map[string]interface{}, keyName string) (int, error) {
	val, ok := configuration[keyName]
	if !ok {
		return 0, errors.New(fmt.Sprintf("key %s not found", keyName))
	}

	valInt, ok := val.(int)
	if !ok {
		return 0, errors.New(fmt.Sprintf("*int value type expected for key %s", keyName))
	}

	return valInt, nil
}

func getBoolKey(configuration map[string]interface{}, keyName string) (bool, error) {
	val, ok := configuration[keyName]
	if !ok {
		return false, errors.New(fmt.Sprintf("key %s not found", keyName))
	}

	valInt, ok := val.(bool)
	if !ok {
		return false, errors.New(fmt.Sprintf("bool value type expected for key %s", keyName))
	}

	return valInt, nil
}

func getIntPointerKey(configuration map[string]interface{}, keyName string) (*int, error) {
	val, ok := configuration[keyName]
	if !ok {
		return nil, nil
	}

	valIntPointer, ok := val.(*int)
	if !ok {
		return nil, errors.New(fmt.Sprintf("*int value type expected for key %s", keyName))
	}

	return valIntPointer, nil
}

func getStringPointerKey(configuration map[string]interface{}, keyName string) (*string, error) {
	val, ok := configuration[keyName]
	if !ok {
		return nil, nil
	}

	valIntPointer, ok := val.(*string)
	if !ok {
		return nil, errors.New(fmt.Sprintf("*string value type expected for key %s", keyName))
	}

	return valIntPointer, nil
}
