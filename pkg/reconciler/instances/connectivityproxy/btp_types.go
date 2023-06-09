package connectivityproxy

import (
	"encoding/json"
	"strconv"
)

type btpConSvc struct {
	CasPath        string `json:"CAs_path"`
	CasSigningPath string `json:"CAs_signing_path"`
	APIPath        string `json:"api_path"`
	TunnelPath     string `json:"tunnel_path"`
	URL            string `json:"url"`
}

func (s *btpConSvc) UnmarshalJSON(src []byte) error {
	sanitized, err := strconv.Unquote(string(src))
	if err != nil {
		return err
	}
	// create alias not to fall into circular unmarshal
	type btpConSvcAlias btpConSvc
	var v btpConSvcAlias

	err = json.Unmarshal([]byte(sanitized), &v)
	if err != nil {
		return err
	}

	*s = btpConSvc(v)
	return nil
}

type btpTags []string

func (s *btpTags) UnmarshalJSON(src []byte) error {
	sanitized, err := strconv.Unquote(string(src))
	if err != nil {
		return err
	}
	// avoid circular unmarshal
	var data []string
	if err := json.Unmarshal([]byte(sanitized), &data); err != nil {
		return err
	}

	*s = data
	return nil
}

type btpSvcKey struct {
	ClientID                    string    `json:"clientid"`
	ClientSecret                string    `json:"clientsecret"`
	ConnectovitySvc             btpConSvc `json:"connectivity_service"`
	CredentialsType             string    `json:"credential-type"`
	InstanceGUID                string    `json:"instance_guid"`
	InstanceName                string    `json:"instance_name"`
	Label                       string    `json:"label"`
	Plan                        string    `json:"plan"`
	SubaccountID                string    `json:"subaccount_id"`
	SubaccountSubdomain         string    `json:"subaccount_subdomain"`
	Tags                        btpTags   `json:"tags"`
	TokenSvcDomain              string    `json:"token_service_domain"`
	TokenSvcURL                 string    `json:"token_service_url"`
	TokenSvcURLPattern          string    `json:"token_service_url_pattern"`
	TokenSVCURLPatternTenantKey string    `json:"token_service_url_pattern_tenant_key"`
	Type                        string    `json:"type"`
	XsAppName                   string    `json:"xsappname"`
}
