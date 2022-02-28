package gardener

import (
	"encoding/json"
	"fmt"
	gardenerTypes "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	apiMachineryRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	SubAccountLabel = "subaccount"
	AccountLabel    = "account"

	LicenceTypeAnnotation = "kcp.provisioner.kyma-project.io/licence-type"
)

type Config keb.GardenerConfig
type GCPProviderConfig keb.GcpProviderConfig
type AzureProviderConfig keb.AzureProviderConfig
type AWSProviderConfig keb.AwsProviderConfig
type ProviderSpecificConfig keb.ProviderSpecificConfig

func (c Config) ToShootTemplate(namespace string, accountId string, subAccountId string, oidcConfig *keb.OidcConfig, dnsInputConfig *keb.DnsConfig) (*gardenerTypes.Shoot, error) {
	enableBasicAuthentication := false

	var seed *string = nil
	if c.Seed != "" {
		seed = stringPtr(c.Seed)
	}
	var purpose *gardenerTypes.ShootPurpose = nil
	if notNilOrEmpty(c.Purpose) {
		p := gardenerTypes.ShootPurpose(*c.Purpose)
		purpose = &p
	}

	var exposureClassName *string = nil

	if notNilOrEmpty(c.ExposureClassName) {
		exposureClassName = c.ExposureClassName
	}

	annotations := make(map[string]string)
	if c.LicenceType != nil {
		annotations[LicenceTypeAnnotation] = *c.LicenceType
	}

	dnsConfig := NewDNSConfig()
	jsonDNSConfig, encodingErr := json.Marshal(dnsConfig)
	if encodingErr != nil {
		return nil, errors.New(fmt.Sprintf("error encoding DNS extension config: %s", encodingErr.Error()))
	}

	certConfig := NewCertConfig()
	jsonCertConfig, encodingErr := json.Marshal(certConfig)
	if encodingErr != nil {
		return nil, errors.New(fmt.Sprintf("error encoding Cert extension config: %s", encodingErr.Error()))
	}

	shoot := &gardenerTypes.Shoot{
		ObjectMeta: v1.ObjectMeta{
			Name:      c.Name,
			Namespace: namespace,
			Labels: map[string]string{
				SubAccountLabel: subAccountId,
				AccountLabel:    accountId,
			},
			Annotations: annotations,
		},
		Spec: gardenerTypes.ShootSpec{
			SecretBindingName: c.TargetSecret,
			SeedName:          seed,
			Region:            c.Region,
			Kubernetes: gardenerTypes.Kubernetes{
				AllowPrivilegedContainers: &c.AllowPrivilegedContainers,
				Version:                   c.KubernetesVersion,
				KubeAPIServer: &gardenerTypes.KubeAPIServerConfig{
					EnableBasicAuthentication: &enableBasicAuthentication,
					OIDCConfig:                gardenerOidcConfig(oidcConfig),
				},
			},
			Networking: gardenerTypes.Networking{
				Type:  "calico",                   // TODO: Default value - we may consider adding it to API
				Nodes: stringPtr("10.250.0.0/19"), // TODO: it is required - provide configuration in API
			},
			Purpose:           purpose,
			ExposureClassName: exposureClassName,
			Maintenance: &gardenerTypes.Maintenance{
				AutoUpdate: &gardenerTypes.MaintenanceAutoUpdate{
					KubernetesVersion:   c.EnableKubernetesVersionAutoUpdate,
					MachineImageVersion: c.EnableMachineImageVersionAutoUpdate,
				},
			},
			DNS: gardenerDnsConfig(dnsInputConfig),
			Extensions: []gardenerTypes.Extension{
				{
					Type:           "shoot-dns-service",
					ProviderConfig: &apiMachineryRuntime.RawExtension{Raw: jsonDNSConfig},
				},
				{
					Type:           "shoot-cert-service",
					ProviderConfig: &apiMachineryRuntime.RawExtension{Raw: jsonCertConfig},
				},
			},
		},
	}

	err := c.AddProviderSpecificConfig(shoot)
	if err != nil {
		return nil, errors.New("error extending shoot config with Provider")
	}

	return shoot, nil
}
func (c Config) AddProviderSpecificConfig(shoot *gardenerTypes.Shoot) error {
	if c.ProviderSpecificConfig.Gcp != nil {
		return GCPProviderConfig(*c.ProviderSpecificConfig.Gcp).ExtendShootConfig(c, shoot)
	} else if c.ProviderSpecificConfig.Azure != nil {
		return AzureProviderConfig(*c.ProviderSpecificConfig.Azure).ExtendShootConfig(c, shoot)
	} else if c.ProviderSpecificConfig.Aws != nil {
		return AWSProviderConfig(*c.ProviderSpecificConfig.Aws).ExtendShootConfig(c, shoot)
	} else {
		return errors.New("invalid provider config")
	}
}

func (c GCPProviderConfig) ExtendShootConfig(gardenerConfig Config, shoot *gardenerTypes.Shoot) error {
	shoot.Spec.CloudProfileName = "gcp"

	workers := []gardenerTypes.Worker{getWorkerConfig(gardenerConfig, c.Zones)}

	gcpInfra := NewGCPInfrastructure(gardenerConfig.WorkerCidr)
	jsonData, err := json.Marshal(gcpInfra)
	if err != nil {
		return errors.New(fmt.Sprintf("error encoding infrastructure config: %s", err.Error()))
	}

	gcpControlPlane := NewGCPControlPlane(c.Zones)
	jsonCPData, err := json.Marshal(gcpControlPlane)
	if err != nil {
		return errors.New(fmt.Sprintf("error encoding control plane config: %s", err.Error()))
	}

	shoot.Spec.Provider = gardenerTypes.Provider{
		Type:                 "gcp",
		ControlPlaneConfig:   &apiMachineryRuntime.RawExtension{Raw: jsonCPData},
		InfrastructureConfig: &apiMachineryRuntime.RawExtension{Raw: jsonData},
		Workers:              workers,
	}

	return nil
}

func (c AzureProviderConfig) ExtendShootConfig(gardenerConfig Config, shoot *gardenerTypes.Shoot) error {
	shoot.Spec.CloudProfileName = "az"

	workers := []gardenerTypes.Worker{getWorkerConfig(gardenerConfig, c.Zones)}

	azInfra := NewAzureInfrastructure(gardenerConfig.WorkerCidr, c)
	jsonData, err := json.Marshal(azInfra)
	if err != nil {
		return errors.New(fmt.Sprintf("error encoding infrastructure config: %s", err.Error()))
	}

	azureControlPlane := NewAzureControlPlane(c.Zones)
	jsonCPData, err := json.Marshal(azureControlPlane)
	if err != nil {
		return errors.New(fmt.Sprintf("error encoding control plane config: %s", err.Error()))
	}

	shoot.Spec.Provider = gardenerTypes.Provider{
		Type:                 "azure",
		ControlPlaneConfig:   &apiMachineryRuntime.RawExtension{Raw: jsonCPData},
		InfrastructureConfig: &apiMachineryRuntime.RawExtension{Raw: jsonData},
		Workers:              workers,
	}

	return nil
}

func (c AWSProviderConfig) ExtendShootConfig(gardenerConfig Config, shoot *gardenerTypes.Shoot) error {
	shoot.Spec.CloudProfileName = "aws"

	zoneNames := getAWSZonesNames(*c.AwsZones)

	workers := []gardenerTypes.Worker{getWorkerConfig(gardenerConfig, zoneNames)}

	awsInfra := NewAWSInfrastructure(keb.AwsProviderConfig(c))
	jsonData, err := json.Marshal(awsInfra)
	if err != nil {
		return errors.New(fmt.Sprintf("error encoding infrastructure config: %s", err.Error()))
	}

	awsControlPlane := NewAWSControlPlane()
	jsonCPData, err := json.Marshal(awsControlPlane)
	if err != nil {
		return errors.New(fmt.Sprintf("error encoding control plane config: %s", err.Error()))
	}

	shoot.Spec.Provider = gardenerTypes.Provider{
		Type:                 "aws",
		ControlPlaneConfig:   &apiMachineryRuntime.RawExtension{Raw: jsonCPData},
		InfrastructureConfig: &apiMachineryRuntime.RawExtension{Raw: jsonData},
		Workers:              workers,
	}

	return nil
}

func getAWSZonesNames(zones []keb.AwsZone) []string {
	zoneNames := make([]string, 0)

	for _, zone := range zones {
		zoneNames = append(zoneNames, zone.Name)
	}
	return zoneNames
}

func getWorkerConfig(gardenerConfig Config, zones []string) gardenerTypes.Worker {
	worker := gardenerTypes.Worker{
		Name:           "cpu-worker-0",
		MaxSurge:       intOrStringPtr(intstr.FromInt(gardenerConfig.MaxSurge)),
		MaxUnavailable: intOrStringPtr(intstr.FromInt(gardenerConfig.MaxUnavailable)),
		Machine:        getMachineConfig(gardenerConfig),
		Maximum:        int32(gardenerConfig.AutoScalerMax),
		Minimum:        int32(gardenerConfig.AutoScalerMin),
		Zones:          zones,
	}

	if gardenerConfig.DiskType != nil && gardenerConfig.VolumeSizeGB != nil {
		worker.Volume = &gardenerTypes.Volume{
			Type:       gardenerConfig.DiskType,
			VolumeSize: fmt.Sprintf("%dGi", *gardenerConfig.VolumeSizeGB),
		}
	}

	return worker
}

func getMachineConfig(config Config) gardenerTypes.Machine {
	machine := gardenerTypes.Machine{
		Type: config.MachineType,
	}
	if notNilOrEmpty(config.MachineImage) {
		machine.Image = &gardenerTypes.ShootMachineImage{
			Name: *config.MachineImage,
		}
		if notNilOrEmpty(config.MachineImageVersion) {
			machine.Image.Version = config.MachineImageVersion
		}
	}
	return machine

}

func gardenerDnsConfig(dnsConfig *keb.DnsConfig) *gardenerTypes.DNS {
	dns := gardenerTypes.DNS{}

	if dnsConfig != nil {
		dns.Domain = &dnsConfig.Domain
		if dnsConfig.Providers != nil {
			for _, v := range *dnsConfig.Providers {
				domainsInclude := &gardenerTypes.DNSIncludeExclude{
					Include: v.DomainsInclude,
				}

				dns.Providers = append(dns.Providers, gardenerTypes.DNSProvider{
					Domains:    domainsInclude,
					Primary:    &v.Primary,
					SecretName: &v.SecretName,
					Type:       &v.Type,
				})
			}
		}

		return &dns
	}

	return nil
}

func gardenerOidcConfig(oidcConfig *keb.OidcConfig) *gardenerTypes.OIDCConfig {
	if oidcConfig != nil {
		return &gardenerTypes.OIDCConfig{
			ClientID:       &oidcConfig.ClientID,
			GroupsClaim:    &oidcConfig.GroupsClaim,
			IssuerURL:      &oidcConfig.IssuerURL,
			SigningAlgs:    oidcConfig.SigningAlgs,
			UsernameClaim:  &oidcConfig.UsernameClaim,
			UsernamePrefix: &oidcConfig.UsernamePrefix,
		}
	}
	return nil
}

type ExtensionProviderConfig struct {
	// ApiVersion is gardener extension api version
	ApiVersion string `json:"apiVersion"`
	// DnsProviderReplication indicates whether dnsProvider replication is on
	DNSProviderReplication *DNSProviderReplication `json:"dnsProviderReplication,omitempty"`
	// ShootIssuers indicates whether shoot Issuers are on
	ShootIssuers *ShootIssuers `json:"shootIssuers,omitempty"`
	// Kind is extension type
	Kind string `json:"kind"`
}

type DNSProviderReplication struct {
	// Enabled indicates whether replication is on
	Enabled bool `json:"enabled"`
}

type ShootIssuers struct {
	// Enabled indicates whether shoot Issuers are on
	Enabled bool `json:"enabled"`
}

func NewDNSConfig() *ExtensionProviderConfig {
	return &ExtensionProviderConfig{
		ApiVersion:             "service.dns.extensions.gardener.cloud/v1alpha1",
		DNSProviderReplication: &DNSProviderReplication{Enabled: true},
		Kind:                   "DNSConfig",
	}
}

func NewCertConfig() *ExtensionProviderConfig {
	return &ExtensionProviderConfig{
		ApiVersion:   "service.cert.extensions.gardener.cloud/v1alpha1",
		ShootIssuers: &ShootIssuers{Enabled: true},
		Kind:         "CertConfig",
	}
}

func notNilOrEmpty(str *string) bool {
	return str != nil && *str != ""
}

// StringPtr returns pointer to given string
func stringPtr(str string) *string {
	return &str
}

// IntOrStringPtr returns pointer to given int or string
func intOrStringPtr(intOrStr intstr.IntOrString) *intstr.IntOrString {
	return &intOrStr
}
