package gardener

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/provisioning/gardener/infrastructure/aws"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/provisioning/gardener/infrastructure/azure"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/provisioning/gardener/infrastructure/gcp"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/provisioning/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	infrastructureConfigKind = "InfrastructureConfig"
	controlPlaneConfigKind   = "ControlPlaneConfig"

	gcpAPIVersion       = "gcp.provider.extensions.gardener.cloud/v1alpha1"
	azureAPIVersion     = "azure.provider.extensions.gardener.cloud/v1alpha1"
	awsAPIVersion       = "aws.provider.extensions.gardener.cloud/v1alpha1"
	openStackApiVersion = "openstack.provider.extensions.gardener.cloud/v1alpha1"
)

func NewGCPInfrastructure(workerCIDR string) *gcp.InfrastructureConfig {
	return &gcp.InfrastructureConfig{
		TypeMeta: v1.TypeMeta{
			Kind:       infrastructureConfigKind,
			APIVersion: gcpAPIVersion,
		},
		Networks: gcp.NetworkConfig{
			Worker:  workerCIDR,
			Workers: util.StringPtr(workerCIDR),
		},
	}
}

func NewGCPControlPlane(zones []string) *gcp.ControlPlaneConfig {
	return &gcp.ControlPlaneConfig{
		TypeMeta: v1.TypeMeta{
			Kind:       controlPlaneConfigKind,
			APIVersion: gcpAPIVersion,
		},
		Zone: zones[0],
	}
}

func NewAzureInfrastructure(workerCIDR string, azConfig AzureProviderConfig) *azure.InfrastructureConfig {
	isZoned := len(azConfig.Zones) > 0
	return &azure.InfrastructureConfig{
		TypeMeta: v1.TypeMeta{
			Kind:       infrastructureConfigKind,
			APIVersion: azureAPIVersion,
		},
		Networks: azure.NetworkConfig{
			Workers: workerCIDR,
			VNet: azure.VNet{
				CIDR: &azConfig.VnetCidr,
			},
		},
		Zoned: isZoned,
	}
}

func NewAzureControlPlane(zones []string) *azure.ControlPlaneConfig {
	return &azure.ControlPlaneConfig{
		TypeMeta: v1.TypeMeta{
			Kind:       controlPlaneConfigKind,
			APIVersion: azureAPIVersion,
		},
	}
}

func NewAWSInfrastructure(awsConfig AWSProviderConfig) *aws.InfrastructureConfig {
	return &aws.InfrastructureConfig{
		TypeMeta: v1.TypeMeta{
			Kind:       infrastructureConfigKind,
			APIVersion: awsAPIVersion,
		},
		Networks: aws.Networks{
			Zones: createAWSZones(awsConfig.AwsZones),
			VPC: aws.VPC{
				CIDR: util.StringPtr(awsConfig.VpcCidr),
			},
		},
	}
}

func createAWSZones(inputZones []*AWSZone) []aws.Zone {
	zones := make([]aws.Zone, 0)

	for _, inputZone := range inputZones {
		zone := aws.Zone{
			Name:     inputZone.Name,
			Internal: inputZone.InternalCidr,
			Public:   inputZone.PublicCidr,
			Workers:  inputZone.WorkerCidr,
		}
		zones = append(zones, zone)
	}
	return zones
}

func NewAWSControlPlane() *aws.ControlPlaneConfig {
	return &aws.ControlPlaneConfig{
		TypeMeta: v1.TypeMeta{
			Kind:       controlPlaneConfigKind,
			APIVersion: awsAPIVersion,
		},
	}
}
