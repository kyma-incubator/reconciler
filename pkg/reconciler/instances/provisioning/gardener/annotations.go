package gardener

import (
	gardenerTypes "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

type ProvisioningState string

func (s ProvisioningState) String() string {
	return string(s)
}

type KymaInstallationState string

func (s KymaInstallationState) String() string {
	return string(s)
}

const (
	runtimeIDAnnotation   string = "kcp.provisioner.kyma-project.io/runtime-id"
	operationIDAnnotation string = "kcp.provisioner.kyma-project.io/operation-id"

	legacyRuntimeIDAnnotation   string = "compass.provisioner.kyma-project.io/runtime-id"
	legacyOperationIDAnnotation string = "compass.provisioner.kyma-project.io/operation-id"
)

func annotate(shoot *gardenerTypes.Shoot, annotation, value string) {
	if shoot.Annotations == nil {
		shoot.Annotations = map[string]string{}
	}

	shoot.Annotations[annotation] = value
}
