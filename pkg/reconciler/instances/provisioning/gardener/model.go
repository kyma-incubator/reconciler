package gardener

import (
	"encoding/json"
	"fmt"
	gardenertypes "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/provisioning/util"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	SubAccountLabel = "subaccount"
	AccountLabel    = "account"

	LicenceTypeAnnotation = "kcp.provisioner.kyma-project.io/licence-type"
)

type DNSConfig struct {
	Domain    string         `json:"domain"`
	Providers []*DNSProvider `json:"providers"`
}

type DNSProvider struct {
	DomainsInclude []string `json:"domainsInclude"`
	Primary        bool     `json:"primary"`
	SecretName     string   `json:"secretName"`
	Type           string   `json:"type"`
}

type GardenerConfig keb.GardenerConfig

func (c GardenerConfig) ToShootTemplate(namespace string, accountId string, subAccountId string, oidcConfig *keb.OidcConfig, dnsInputConfig *keb.DnsConfig) (*gardenertypes.Shoot, error) {
	enableBasicAuthentication := false

	var seed *string = nil
	if c.Seed != "" {
		seed = util.StringPtr(c.Seed)
	}
	var purpose *gardenertypes.ShootPurpose = nil
	if util.NotNilOrEmpty(c.Purpose) {
		p := gardenertypes.ShootPurpose(*c.Purpose)
		purpose = &p
	}

	var exposureClassName *string = nil

	if util.NotNilOrEmpty(c.ExposureClassName) {
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

	shoot := &gardenertypes.Shoot{
		ObjectMeta: v1.ObjectMeta{
			Name:      c.Name,
			Namespace: namespace,
			Labels: map[string]string{
				SubAccountLabel: subAccountId,
				AccountLabel:    accountId,
			},
			Annotations: annotations,
		},
		Spec: gardenertypes.ShootSpec{
			SecretBindingName: c.TargetSecret,
			SeedName:          seed,
			Region:            c.Region,
			Kubernetes: gardenertypes.Kubernetes{
				AllowPrivilegedContainers: &c.AllowPrivilegedContainers,
				Version:                   c.KubernetesVersion,
				KubeAPIServer: &gardenertypes.KubeAPIServerConfig{
					EnableBasicAuthentication: &enableBasicAuthentication,
					OIDCConfig:                gardenerOidcConfig(oidcConfig),
				},
			},
			Networking: gardenertypes.Networking{
				Type:  "calico",                        // Default value - we may consider adding it to API (if Hydroform will support it)
				Nodes: util.StringPtr("10.250.0.0/19"), // TODO: it is required - provide configuration in API (when Hydroform will support it)
			},
			Purpose:           purpose,
			ExposureClassName: exposureClassName,
			Maintenance: &gardenertypes.Maintenance{
				AutoUpdate: &gardenertypes.MaintenanceAutoUpdate{
					KubernetesVersion:   c.EnableKubernetesVersionAutoUpdate,
					MachineImageVersion: c.EnableMachineImageVersionAutoUpdate,
				},
			},
			DNS: gardenerDnsConfig(dnsInputConfig),
			Extensions: []gardenertypes.Extension{
				{
					Type:           "shoot-dns-service",
					ProviderConfig: &apimachineryRuntime.RawExtension{Raw: jsonDNSConfig},
				},
				{
					Type:           "shoot-cert-service",
					ProviderConfig: &apimachineryRuntime.RawExtension{Raw: jsonCertConfig},
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
func (c GardenerConfig) AddProviderSpecificConfig(shoot *gardenertypes.Shoot) error {
	if c.ProviderSpecificConfig.Gcp != nil {
		return GCPProviderConfig(*c.ProviderSpecificConfig.Gcp).ExtendShootConfig(c, shoot)
	} else if c.ProviderSpecificConfig.Azure != nil {
		return AzureProviderConfig(*c.ProviderSpecificConfig.Azure).ExtendShootConfig(c, shoot)
	} else if c.ProviderSpecificConfig.Aws != nil {
		return AWSProviderConfig(*c.ProviderSpecificConfig.Aws).ExtendShootConfig(c, shoot)
	} else {
		return errors.New("invalid provider config")
	}
	return nil
}

type GCPProviderConfig keb.GcpProviderConfig
type AzureProviderConfig keb.AzureProviderConfig
type AWSProviderConfig keb.AwsProviderConfig
type ProviderSpecificConfig keb.ProviderSpecificConfig

func (c GCPProviderConfig) ExtendShootConfig(gardenerConfig GardenerConfig, shoot *gardenertypes.Shoot) error {
	shoot.Spec.CloudProfileName = "gcp"

	workers := []gardenertypes.Worker{getWorkerConfig(gardenerConfig, c.Zones)}

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

	shoot.Spec.Provider = gardenertypes.Provider{
		Type:                 "gcp",
		ControlPlaneConfig:   &apimachineryRuntime.RawExtension{Raw: jsonCPData},
		InfrastructureConfig: &apimachineryRuntime.RawExtension{Raw: jsonData},
		Workers:              workers,
	}

	return nil
}

func (c AzureProviderConfig) ExtendShootConfig(gardenerConfig GardenerConfig, shoot *gardenertypes.Shoot) error {
	shoot.Spec.CloudProfileName = "az"

	workers := []gardenertypes.Worker{getWorkerConfig(gardenerConfig, c.Zones)}

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

	shoot.Spec.Provider = gardenertypes.Provider{
		Type:                 "azure",
		ControlPlaneConfig:   &apimachineryRuntime.RawExtension{Raw: jsonCPData},
		InfrastructureConfig: &apimachineryRuntime.RawExtension{Raw: jsonData},
		Workers:              workers,
	}

	return nil
}

func (c AWSProviderConfig) ExtendShootConfig(gardenerConfig GardenerConfig, shoot *gardenertypes.Shoot) error {
	shoot.Spec.CloudProfileName = "aws"

	zoneNames := getAWSZonesNames(c.AwsZones)

	workers := []gardenertypes.Worker{getWorkerConfig(gardenerConfig, zoneNames)}

	awsInfra := NewAWSInfrastructure(c)
	jsonData, err := json.Marshal(awsInfra)
	if err != nil {
		return errors.New(fmt.Sprintf("error encoding infrastructure config: %s", err.Error()))
	}

	awsControlPlane := NewAWSControlPlane()
	jsonCPData, err := json.Marshal(awsControlPlane)
	if err != nil {
		return errors.New(fmt.Sprintf("error encoding control plane config: %s", err.Error()))
	}

	shoot.Spec.Provider = gardenertypes.Provider{
		Type:                 "aws",
		ControlPlaneConfig:   &apimachineryRuntime.RawExtension{Raw: jsonCPData},
		InfrastructureConfig: &apimachineryRuntime.RawExtension{Raw: jsonData},
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

func getWorkerConfig(gardenerConfig GardenerConfig, zones []string) gardenertypes.Worker {
	worker := gardenertypes.Worker{
		Name:           "cpu-worker-0",
		MaxSurge:       util.IntOrStringPtr(intstr.FromInt(gardenerConfig.MaxSurge)),
		MaxUnavailable: util.IntOrStringPtr(intstr.FromInt(gardenerConfig.MaxUnavailable)),
		Machine:        getMachineConfig(gardenerConfig),
		Maximum:        int32(gardenerConfig.AutoScalerMax),
		Minimum:        int32(gardenerConfig.AutoScalerMin),
		Zones:          zones,
	}

	if gardenerConfig.DiskType != nil && gardenerConfig.VolumeSizeGB != nil {
		worker.Volume = &gardenertypes.Volume{
			Type:       gardenerConfig.DiskType,
			VolumeSize: fmt.Sprintf("%dGi", *gardenerConfig.VolumeSizeGB),
		}
	}

	return worker
}

func getMachineConfig(config GardenerConfig) gardenertypes.Machine {
	machine := gardenertypes.Machine{
		Type: config.MachineType,
	}
	if util.NotNilOrEmpty(config.MachineImage) {
		machine.Image = &gardenertypes.ShootMachineImage{
			Name: *config.MachineImage,
		}
		if util.NotNilOrEmpty(config.MachineImageVersion) {
			machine.Image.Version = config.MachineImageVersion
		}
	}
	return machine

}

func gardenerDnsConfig(dnsConfig *keb.DnsConfig) *gardenertypes.DNS {
	dns := gardenertypes.DNS{}

	if dnsConfig != nil {
		dns.Domain = &dnsConfig.Domain
		if dnsConfig.Providers != nil {
			for _, v := range *dnsConfig.Providers {
				domainsInclude := &gardenertypes.DNSIncludeExclude{
					Include: v.DomainsInclude,
				}

				dns.Providers = append(dns.Providers, gardenertypes.DNSProvider{
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

func gardenerOidcConfig(oidcConfig *keb.OidcConfig) *gardenertypes.OIDCConfig {
	if oidcConfig != nil {
		return &gardenertypes.OIDCConfig{
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
