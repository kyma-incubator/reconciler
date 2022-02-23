package gardener

import (
	"encoding/json"
	"fmt"
	gardenertypes "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/provisioning/util"
	"github.com/kyma-project/control-plane/components/provisioner/pkg/gqlschema"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
)

const (
	SubAccountLabel = "subaccount"
	AccountLabel    = "account"

	LicenceTypeAnnotation = "kcp.provisioner.kyma-project.io/licence-type"
)

type OIDCConfig struct {
	ClientID       string   `json:"clientID"`
	GroupsClaim    string   `json:"groupsClaim"`
	IssuerURL      string   `json:"issuerURL"`
	SigningAlgs    []string `json:"signingAlgs"`
	UsernameClaim  string   `json:"usernameClaim"`
	UsernamePrefix string   `json:"usernamePrefix"`
}

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

type GardenerConfig struct {
	ID                                  string
	ClusterID                           string
	Name                                string
	ProjectName                         string
	KubernetesVersion                   string
	VolumeSizeGB                        *int
	DiskType                            *string
	MachineType                         string
	MachineImage                        *string
	MachineImageVersion                 *string
	Provider                            string
	Purpose                             *string
	LicenceType                         *string
	Seed                                string
	TargetSecret                        string
	Region                              string
	WorkerCidr                          string
	AutoScalerMin                       int
	AutoScalerMax                       int
	MaxSurge                            int
	MaxUnavailable                      int
	EnableKubernetesVersionAutoUpdate   bool
	EnableMachineImageVersionAutoUpdate bool
	AllowPrivilegedContainers           bool
	OIDCConfig                          *OIDCConfig
	DNSConfig                           *DNSConfig
	ExposureClassName                   *string
	GardenerProviderConfig              GardenerProviderConfig
}

func (c GardenerConfig) ToShootTemplate(namespace string, accountId string, subAccountId string, oidcConfig *OIDCConfig, dnsInputConfig *DNSConfig) (*gardenertypes.Shoot, error) {
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

	err := c.GardenerProviderConfig.ExtendShootConfig(c, shoot)
	if err != nil {
		return nil, errors.New("error extending shoot config with Provider")
	}

	return shoot, nil
}

type GardenerProviderConfig interface {
	RawJSON() string
	AsProviderSpecificConfig() gqlschema.ProviderSpecificConfig
	ExtendShootConfig(gardenerConfig GardenerConfig, shoot *gardenertypes.Shoot) error
	EditShootConfig(gardenerConfig GardenerConfig, shoot *gardenertypes.Shoot) error
}

func gardenerDnsConfig(dnsConfig *DNSConfig) *gardenertypes.DNS {
	dns := gardenertypes.DNS{}

	if dnsConfig != nil {
		dns.Domain = &dnsConfig.Domain
		if len(dnsConfig.Providers) != 0 {
			for _, v := range dnsConfig.Providers {
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

func gardenerOidcConfig(oidcConfig *OIDCConfig) *gardenertypes.OIDCConfig {
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
