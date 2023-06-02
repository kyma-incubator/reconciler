package connectivityproxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"

	v1 "k8s.io/api/core/v1"
)

type encodedString string

func (s *encodedString) UnmarshalJSON(src []byte) error {
	sanitized := bytes.Trim(src, "\"")
	raw, err := base64.StdEncoding.DecodeString(string(sanitized))
	if err != nil {
		return err
	}

	*s = encodedString(raw)
	return nil
}

type encodedSlice []encodedString

func (s *encodedSlice) UnmarshalJSON(src []byte) error {
	sanitized := bytes.Trim(src, "\"")
	decodedSrc, err := base64.StdEncoding.DecodeString(string(sanitized))
	if err != nil {
		return err
	}

	var data []string
	if err := json.Unmarshal(decodedSrc, &data); err != nil {
		return err
	}

	out := []encodedString{}
	for _, item := range data {
		out = append(out, encodedString(item))
	}
	*s = out
	return nil
}

type encodedConSvc struct {
	CasPath        string `json:"CAs_path"`
	CasSigningPath string `json:"CAs_signing_path"`
	APIPath        string `json:"api_path"`
	TunnelPath     string `json:"tunnel_path"`
	URL            string `json:"url"`
}

func (s *encodedConSvc) UnmarshalJSON(src []byte) error {
	sanitized := bytes.Trim(src, "\"")
	raw, err := base64.StdEncoding.DecodeString(string(sanitized))
	if err != nil {
		return err
	}
	// create alias not to fall into circular unmarshal
	type connectivitySvcAlias encodedConSvc
	var cpAlias connectivitySvcAlias

	if err := json.Unmarshal(raw, &cpAlias); err != nil {
		return err
	}

	*s = encodedConSvc(cpAlias)
	return nil
}

type svcKey struct {
	ClientID                    encodedString `json:"clientid"`
	ClientSecret                encodedString `json:"clientsecret"`
	ConnectovitySvc             encodedConSvc `json:"connectivity_service"`
	CredentialsType             encodedString `json:"credential-type"`
	InstanceGUID                encodedString `json:"instance_guid"`
	InstanceName                encodedString `json:"instance_name"`
	Label                       encodedString `json:"label"`
	Plan                        encodedString `json:"plan"`
	SubaccountID                encodedString `json:"subaccount_id"`
	SubaccountSubdomain         encodedString `json:"subaccount_subdomain"`
	Tags                        encodedSlice  `json:"tags"`
	TokenSvcDomain              encodedString `json:"token_service_domain"`
	TokenSvcURL                 encodedString `json:"token_service_url"`
	TokenSvcURLPattern          encodedString `json:"token_service_url_pattern"`
	TokenSVCURLPatternTenantKey encodedString `json:"token_service_url_pattern_tenant_key"`
	Type                        encodedString `json:"type"`
	XsAppName                   encodedString `json:"xsappname"`
}

func (k *svcKey) fromSecret(s *v1.Secret) error {
	data, err := json.Marshal(s.Data)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, k)
}
